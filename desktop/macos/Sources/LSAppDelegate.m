#import "LSAppDelegate.h"

#import "LSBundleLocator.h"
#import "LSDashboardCoordinator.h"
#import "LSDesktopCoordinator.h"

@interface LSAppDelegate ()

@property (nonatomic, strong, nullable) LSDesktopCoordinator *menuAgentCoordinator;
@property (nonatomic, strong, nullable) LSDashboardCoordinator *dashboardCoordinator;
@property (nonatomic, assign) BOOL menuAgentMode;

@end

@implementation LSAppDelegate

- (void)applicationDidFinishLaunching:(__unused NSNotification *)notification {
    self.menuAgentMode = [LSBundleLocator isMenuAgentBundle];

    if (self.menuAgentMode) {
        self.menuAgentCoordinator = [[LSDesktopCoordinator alloc] init];
        [self.menuAgentCoordinator start];
        return;
    }

    self.dashboardCoordinator = [[LSDashboardCoordinator alloc] init];
    [self.dashboardCoordinator start];
}

- (BOOL)applicationShouldHandleReopen:(__unused NSApplication *)sender hasVisibleWindows:(__unused BOOL)flag {
    if (self.menuAgentMode) {
        return NO;
    }
    return [self.dashboardCoordinator handleApplicationReopen];
}

- (void)applicationWillTerminate:(__unused NSNotification *)notification {
    if (self.menuAgentMode) {
        [self.menuAgentCoordinator prepareForTermination];
        return;
    }
    [self.dashboardCoordinator prepareForTermination];
}

- (BOOL)applicationShouldTerminateAfterLastWindowClosed:(__unused NSApplication *)sender {
    if (self.menuAgentMode) {
        return NO;
    }
    return [self.dashboardCoordinator shouldTerminateAfterLastWindowClosed];
}

@end
