#import "LSDesktopCoordinator.h"

#import "LSBundleLocator.h"
#import "LSOverviewFetcher.h"
#import "LSRuntimeConfig.h"
#import "LSServiceController.h"
#import "LSStatusItemController.h"

@interface LSDesktopCoordinator ()

@property (nonatomic, strong) LSStatusItemController *statusItemController;

@property (nonatomic, strong, nullable) LSRuntimeConfig *runtime;
@property (nonatomic, strong, nullable) LSServiceController *serviceController;
@property (nonatomic, strong, nullable) LSOverviewFetcher *overviewFetcher;
@property (nonatomic) LSServiceStatus currentStatus;
@property (nonatomic, copy) NSString *currentMessage;

@end

@implementation LSDesktopCoordinator

- (instancetype)init {
    self = [super init];
    if (!self) {
        return nil;
    }

    _currentStatus = LSServiceStatusIdle;
    _currentMessage = @"";
    _statusItemController = [[LSStatusItemController alloc] init];

    __weak typeof(self) weakSelf = self;
    _statusItemController.onOpenDashboard = ^{
        [weakSelf openDashboard];
    };
    _statusItemController.onRetry = ^{
        [weakSelf startService];
    };
    _statusItemController.onQuit = ^{
        [weakSelf quitLlamaSitter];
    };
    _statusItemController.onRefreshRequested = ^{
        [weakSelf refreshOverviewIfNeeded];
    };

    return self;
}

- (void)start {
    [self.statusItemController updateLocalStatus:LSServiceStatusStarting message:@"Preparing the background agent."];
    [self startService];
}

- (void)prepareForTermination {
    [self.statusItemController dismissPopover];
    [self terminateDashboardApplications];
    [self.serviceController stop];
}

- (BOOL)ensureDesktopRuntime {
    if (self.runtime && self.serviceController && self.overviewFetcher) {
        return YES;
    }

    NSError *runtimeError = nil;
    LSRuntimeConfig *runtime = [[LSRuntimeConfig alloc] initWithError:&runtimeError];
    if (!runtime) {
        NSString *message = runtimeError.localizedDescription ?: @"Runtime configuration is unavailable.";
        self.currentStatus = LSServiceStatusFailed;
        self.currentMessage = message;
        [self.statusItemController updateLocalStatus:LSServiceStatusFailed message:message];
        return NO;
    }

    self.runtime = runtime;
    self.overviewFetcher = [[LSOverviewFetcher alloc] initWithBaseURL:runtime.uiBaseURL];

    LSServiceController *serviceController = [[LSServiceController alloc] initWithRuntime:runtime];
    __weak typeof(self) weakSelf = self;
    serviceController.onStatusChange = ^(LSServiceStatus status, NSURL *dashboardURL, NSString *message) {
        [weakSelf handleServiceStatus:status dashboardURL:dashboardURL message:message];
    };
    self.serviceController = serviceController;

    return YES;
}

- (void)startService {
    if (![self ensureDesktopRuntime]) {
        return;
    }

    NSString *message = [NSString stringWithFormat:
                         @"Launching the bundled local service and waiting for metrics on %@.",
                         self.runtime.uiListenAddr];
    [self.statusItemController updateLocalStatus:LSServiceStatusStarting message:message];
    self.currentStatus = LSServiceStatusStarting;
    self.currentMessage = message;
    [self.statusItemController applyOverviewSnapshot:nil];
    [self.serviceController start];
}

- (void)handleServiceStatus:(LSServiceStatus)status dashboardURL:(NSURL * _Nullable)dashboardURL message:(NSString * _Nullable)message {
    self.currentStatus = status;
    self.currentMessage = message ?: @"";
    [self.statusItemController updateLocalStatus:status message:message];

    switch (status) {
        case LSServiceStatusIdle:
            [self.statusItemController applyOverviewSnapshot:nil];
            break;
        case LSServiceStatusStarting: {
            [self.statusItemController applyOverviewSnapshot:nil];
            break;
        }
        case LSServiceStatusReady:
            [self refreshOverviewIfNeeded];
            break;
        case LSServiceStatusFailed:
            [self.statusItemController applyOverviewSnapshot:nil];
            break;
    }
}

- (void)refreshOverviewIfNeeded {
    if (self.currentStatus != LSServiceStatusReady || !self.overviewFetcher) {
        [self.statusItemController updateLocalStatus:self.currentStatus message:self.currentMessage];
        return;
    }

    LSServiceStatus serviceStatus = self.currentStatus;
    NSString *errorMessage = self.currentMessage;

    __weak typeof(self) weakSelf = self;
    [self.overviewFetcher fetchSnapshotForServiceStatus:serviceStatus
                                           errorMessage:errorMessage
                                             completion:^(LSOverviewSnapshot * _Nullable snapshot, NSError * _Nullable error) {
        typeof(self) strongSelf = weakSelf;
        if (!strongSelf || strongSelf.currentStatus != LSServiceStatusReady) {
            return;
        }

        if (snapshot) {
            [strongSelf.statusItemController applyOverviewSnapshot:snapshot];
        } else if (error) {
            [strongSelf.statusItemController updateLocalStatus:LSServiceStatusFailed message:error.localizedDescription];
        }
    }];
}

- (void)openDashboard {
    NSURL *dashboardURL = [LSBundleLocator dashboardAppURL];
    if (!dashboardURL) {
        return;
    }

    NSWorkspaceOpenConfiguration *configuration = [[NSWorkspaceOpenConfiguration alloc] init];
    configuration.activates = YES;
    [[NSWorkspace sharedWorkspace] openApplicationAtURL:dashboardURL
                                          configuration:configuration
                                      completionHandler:nil];
}

- (void)quitLlamaSitter {
    [self prepareForTermination];
    [NSApp terminate:nil];
}

- (void)terminateDashboardApplications {
    for (NSRunningApplication *application in [LSBundleLocator runningDashboardApplications]) {
        [application terminate];
    }

    NSDate *deadline = [NSDate dateWithTimeIntervalSinceNow:1.0];
    while (deadline.timeIntervalSinceNow > 0) {
        NSArray<NSRunningApplication *> *applications = [LSBundleLocator runningDashboardApplications];
        if (applications.count == 0) {
            break;
        }
        [NSThread sleepForTimeInterval:0.05];
    }

    for (NSRunningApplication *application in [LSBundleLocator runningDashboardApplications]) {
        [application forceTerminate];
    }
}

@end
