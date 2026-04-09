#import "LSServiceController.h"

#import "LSPortCheck.h"

@interface LSServiceController ()

@property (nonatomic, strong) LSRuntimeConfig *runtime;
@property (nonatomic, readwrite) LSServiceStatus status;
@property (nonatomic, strong, nullable) NSTask *task;
@property (nonatomic, strong, nullable) NSFileHandle *logHandle;
@property (nonatomic, assign) BOOL intentionalStop;
@property (nonatomic, assign) BOOL attachedToExternalService;

@end

@implementation LSServiceController

- (instancetype)initWithRuntime:(LSRuntimeConfig *)runtime {
    self = [super init];
    if (!self) {
        return nil;
    }

    _runtime = runtime;
    _status = LSServiceStatusIdle;
    return self;
}

- (void)dealloc {
    [self stop];
}

- (void)start {
    [self stopTaskIfNeeded];
    self.attachedToExternalService = NO;

    NSArray<NSString *> *occupied = [LSPortCheck unavailableAddressesInAddresses:@[
        self.runtime.proxyListenAddr,
        self.runtime.uiListenAddr,
    ]];
    if (occupied.count > 0) {
        NSString *message = [NSString stringWithFormat:
                             @"Connecting to the LlamaSitter service already using:\n%@",
                             [occupied componentsJoinedByString:@"\n"]];
        [self updateStatus:LSServiceStatusStarting dashboardURL:nil message:message];
        self.attachedToExternalService = YES;
        [self waitForReadyForTask:nil deadlineSeconds:20.0 startupDescription:@"the existing LlamaSitter service"];
        return;
    }

    if (self.attachOnly) {
        NSString *message = [NSString stringWithFormat:
                             @"Waiting for a LlamaSitter service started with %@.",
                             self.runtime.configURL.path];
        [self updateStatus:LSServiceStatusStarting dashboardURL:nil message:message];
        [self waitForReadyForTask:nil deadlineSeconds:20.0 startupDescription:@"the externally started LlamaSitter service"];
        return;
    }

    if (![[NSFileManager defaultManager] isExecutableFileAtPath:self.runtime.backendExecutableURL.path]) {
        NSString *message = [NSString stringWithFormat:
                             @"Bundled backend executable is missing or not executable at %@.",
                             self.runtime.backendExecutableURL.path];
        [self updateStatus:LSServiceStatusFailed dashboardURL:nil message:message];
        return;
    }

    NSError *logError = nil;
    NSFileHandle *logHandle = [self.runtime openCombinedLogHandleWithError:&logError];
    if (!logHandle) {
        NSString *message = [NSString stringWithFormat:
                             @"Failed to open the backend log.\n\n%@",
                             logError.localizedDescription ?: @"Unknown logging error."];
        [self updateStatus:LSServiceStatusFailed dashboardURL:nil message:message];
        return;
    }

    NSTask *task = [[NSTask alloc] init];
    task.executableURL = self.runtime.backendExecutableURL;
    task.arguments = @[@"serve", @"--config", self.runtime.configURL.path];
    task.standardOutput = logHandle;
    task.standardError = logHandle;
    NSMutableDictionary<NSString *, NSString *> *environment = [[[NSProcessInfo processInfo] environment] mutableCopy];
    environment[@"LLAMASITTER_DESKTOP_MANAGED"] = @"1";
    task.environment = environment;

    __weak typeof(self) weakSelf = self;
    task.terminationHandler = ^(NSTask *finishedTask) {
        dispatch_async(dispatch_get_main_queue(), ^{
            typeof(self) strongSelf = weakSelf;
            if (!strongSelf || finishedTask != strongSelf.task) {
                return;
            }

            BOOL intentionalStop = strongSelf.intentionalStop;
            NSString *reason = finishedTask.terminationReason == NSTaskTerminationReasonExit
                                   ? [NSString stringWithFormat:@"exit code %d", finishedTask.terminationStatus]
                                   : [NSString stringWithFormat:@"signal %d", finishedTask.terminationStatus];

            [strongSelf cleanupProcessResources];

            if (intentionalStop) {
                [strongSelf updateStatus:LSServiceStatusIdle dashboardURL:nil message:nil];
                return;
            }

            NSString *message = [NSString stringWithFormat:
                                 @"The bundled LlamaSitter service stopped unexpectedly (%@).\n\n"
                                 @"Check the backend log at:\n"
                                 @"%@",
                                 reason,
                                 strongSelf.runtime.stdoutLogURL.path];
            [strongSelf updateStatus:LSServiceStatusFailed dashboardURL:nil message:message];
        });
    };

    self.intentionalStop = NO;
    self.logHandle = logHandle;

    NSError *launchError = nil;
    if (![task launchAndReturnError:&launchError]) {
        [self cleanupProcessResources];
        NSString *message = [NSString stringWithFormat:
                             @"Failed to start the bundled LlamaSitter service.\n\n%@",
                             launchError.localizedDescription ?: @"Unknown launch error."];
        [self updateStatus:LSServiceStatusFailed dashboardURL:nil message:message];
        return;
    }

    self.task = task;
    [self updateStatus:LSServiceStatusStarting dashboardURL:nil message:nil];
    [self waitForReadyForTask:task deadlineSeconds:20.0 startupDescription:@"the bundled LlamaSitter service"];
}

- (void)stop {
    self.intentionalStop = YES;
    [self stopTaskIfNeeded];
    self.attachedToExternalService = NO;
    [self updateStatus:LSServiceStatusIdle dashboardURL:nil message:nil];
}

- (void)stopTaskIfNeeded {
    NSTask *task = self.task;
    if (!task) {
        [self cleanupProcessResources];
        return;
    }

    task.terminationHandler = nil;
    if (task.running) {
        [task terminate];

        NSDate *deadline = [NSDate dateWithTimeIntervalSinceNow:1.0];
        while (task.running && deadline.timeIntervalSinceNow > 0) {
            [NSThread sleepForTimeInterval:0.05];
        }

        if (task.running) {
            [task interrupt];
            [task waitUntilExit];
        }
    }

    [self cleanupProcessResources];
}

- (void)cleanupProcessResources {
    [self.logHandle closeFile];
    self.logHandle = nil;
    self.task = nil;
}

- (void)waitForReadyForTask:(nullable NSTask *)task deadlineSeconds:(NSTimeInterval)deadlineSeconds startupDescription:(NSString *)startupDescription {
    NSURL *readyURL = self.runtime.readyURL;
    NSURL *dashboardURL = self.runtime.uiBaseURL;
    NSDate *deadline = [NSDate dateWithTimeIntervalSinceNow:deadlineSeconds];

    __weak typeof(self) weakSelf = self;
    dispatch_async(dispatch_get_global_queue(QOS_CLASS_USER_INITIATED, 0), ^{
        while (deadline.timeIntervalSinceNow > 0) {
            typeof(self) strongSelf = weakSelf;
            if (!strongSelf) {
                return;
            }

            if (task && (task != strongSelf.task || !task.running)) {
                return;
            }

            if ([strongSelf isReadyAtURL:readyURL]) {
                dispatch_async(dispatch_get_main_queue(), ^{
                    typeof(self) innerSelf = weakSelf;
                    if (!innerSelf) {
                        return;
                    }
                    if (task && task != innerSelf.task) {
                        return;
                    }
                    [innerSelf updateStatus:LSServiceStatusReady dashboardURL:dashboardURL message:nil];
                });
                return;
            }

            [NSThread sleepForTimeInterval:0.25];
        }

        dispatch_async(dispatch_get_main_queue(), ^{
            typeof(self) strongSelf = weakSelf;
            if (!strongSelf) {
                return;
            }
            if (task && (task != strongSelf.task || !task.running)) {
                return;
            }

            NSString *message = nil;
            if (task) {
                message = [NSString stringWithFormat:
                           @"LlamaSitter did not become ready from %@ within %.0f seconds.\n\n"
                           @"Check the backend log at:\n"
                           @"%@",
                           startupDescription,
                           deadlineSeconds,
                           strongSelf.runtime.stdoutLogURL.path];
            } else {
                message = [NSString stringWithFormat:
                           @"LlamaSitter did not become ready from %@ within %.0f seconds.\n\n"
                           @"Confirm the service is running with:\n"
                           @"%@\n\n"
                           @"and that the UI listener at %@ is reachable.",
                           startupDescription,
                           deadlineSeconds,
                           strongSelf.runtime.configURL.path,
                           strongSelf.runtime.uiListenAddr];
            }
            strongSelf.attachedToExternalService = NO;
            [strongSelf updateStatus:LSServiceStatusFailed dashboardURL:nil message:message];
        });
    });
}

- (BOOL)isReadyAtURL:(NSURL *)url {
    NSMutableURLRequest *request = [NSMutableURLRequest requestWithURL:url];
    request.timeoutInterval = 1.0;

    dispatch_semaphore_t semaphore = dispatch_semaphore_create(0);
    __block BOOL isReady = NO;

    NSURLSessionDataTask *task = [[NSURLSession sharedSession] dataTaskWithRequest:request
                                                                 completionHandler:^(__unused NSData *data,
                                                                                     NSURLResponse *response,
                                                                                     __unused NSError *error) {
        if ([response isKindOfClass:[NSHTTPURLResponse class]]) {
            NSHTTPURLResponse *httpResponse = (NSHTTPURLResponse *)response;
            if (httpResponse.statusCode == 200) {
                isReady = YES;
            }
        }
        dispatch_semaphore_signal(semaphore);
    }];
    [task resume];

    dispatch_semaphore_wait(semaphore, dispatch_time(DISPATCH_TIME_NOW, (int64_t)(1.5 * NSEC_PER_SEC)));
    return isReady;
}

- (void)updateStatus:(LSServiceStatus)status dashboardURL:(NSURL * _Nullable)dashboardURL message:(NSString * _Nullable)message {
    self.status = status;
    if (self.onStatusChange) {
        self.onStatusChange(status, dashboardURL, message);
    }
}

@end
