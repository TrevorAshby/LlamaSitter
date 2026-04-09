#import "LSDashboardCoordinator.h"

#import "LSBundleLocator.h"
#import "LSMainWindowController.h"
#import "LSRuntimeConfig.h"

@interface LSDashboardCoordinator ()

@property (nonatomic, strong) LSMainWindowController *windowController;
@property (nonatomic, strong, nullable) LSRuntimeConfig *runtime;
@property (nonatomic, assign) BOOL launchInFlight;

@end

@implementation LSDashboardCoordinator

- (instancetype)init {
    self = [super init];
    if (!self) {
        return nil;
    }

    _windowController = [[LSMainWindowController alloc] init];
    __weak typeof(self) weakSelf = self;
    _windowController.onRetry = ^{
        [weakSelf start];
    };
    return self;
}

- (void)start {
    NSError *runtimeError = nil;
    LSRuntimeConfig *runtime = [[LSRuntimeConfig alloc] initWithError:&runtimeError];
    if (!runtime) {
        [self.windowController showErrorMessage:runtimeError.localizedDescription ?: @"Runtime configuration is unavailable."];
        [self.windowController restoreAndFocus];
        return;
    }

    self.runtime = runtime;
    [self.windowController showLoadingWithTitle:@"Opening Dashboard"
                                        message:[NSString stringWithFormat:
                                                 @"Starting the background agent if needed and waiting for metrics on %@.",
                                                 runtime.uiListenAddr]];
    [self.windowController restoreAndFocus];

    [self ensureMenuAgentRunningWithCompletion:^(NSError * _Nullable error) {
        if (error) {
            [self.windowController showErrorMessage:error.localizedDescription ?: @"Unable to launch the background agent."];
            [self.windowController restoreAndFocus];
            return;
        }

        [self waitForReady];
    }];
}

- (BOOL)handleApplicationReopen {
    [self.windowController restoreAndFocus];
    return YES;
}

- (void)prepareForTermination {
}

- (BOOL)shouldTerminateAfterLastWindowClosed {
    return YES;
}

- (void)ensureMenuAgentRunningWithCompletion:(void (^)(NSError * _Nullable error))completion {
    if (self.launchInFlight) {
        completion(nil);
        return;
    }

    if ([LSBundleLocator isMenuAgentRunning]) {
        completion(nil);
        return;
    }

    NSURL *menuAgentURL = [LSBundleLocator embeddedMenuAgentAppURL];
    if (!menuAgentURL) {
        NSError *error = [NSError errorWithDomain:@"LlamaSitterDashboard"
                                             code:1
                                         userInfo:@{NSLocalizedDescriptionKey: @"Unable to locate the embedded background agent."}];
        completion(error);
        return;
    }

    self.launchInFlight = YES;
    NSWorkspaceOpenConfiguration *configuration = [[NSWorkspaceOpenConfiguration alloc] init];
    configuration.activates = NO;

    [[NSWorkspace sharedWorkspace] openApplicationAtURL:menuAgentURL
                                          configuration:configuration
                                      completionHandler:^(__unused NSRunningApplication *app, NSError *error) {
        self.launchInFlight = NO;
        completion(error);
    }];
}

- (void)waitForReady {
    if (!self.runtime) {
        [self.windowController showErrorMessage:@"Runtime configuration is unavailable."];
        return;
    }

    NSURL *readyURL = self.runtime.readyURL;
    NSURL *dashboardURL = self.runtime.uiBaseURL;
    NSDate *deadline = [NSDate dateWithTimeIntervalSinceNow:20.0];

    dispatch_async(dispatch_get_global_queue(QOS_CLASS_USER_INITIATED, 0), ^{
        while (deadline.timeIntervalSinceNow > 0) {
            if ([self isReadyAtURL:readyURL]) {
                dispatch_async(dispatch_get_main_queue(), ^{
                    [self.windowController showDashboardURL:dashboardURL];
                    [self.windowController restoreAndFocus];
                });
                return;
            }

            [NSThread sleepForTimeInterval:0.25];
        }

        dispatch_async(dispatch_get_main_queue(), ^{
            NSString *message = [NSString stringWithFormat:
                                 @"The background LlamaSitter agent did not become ready within 20 seconds.\n\n"
                                 @"Check the app log at:\n"
                                 @"%@",
                                 self.runtime.appLogURL.path];
            [self.windowController showErrorMessage:message];
            [self.windowController restoreAndFocus];
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

@end
