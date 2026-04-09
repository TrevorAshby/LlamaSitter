#import "LSBundleLocator.h"

#import <AppKit/AppKit.h>

NSString *const LSDashboardBundleIdentifier = @"com.trevorashby.LlamaSitter";
NSString *const LSMenuAgentBundleIdentifier = @"com.trevorashby.LlamaSitter.MenuAgent";

@implementation LSBundleLocator

+ (BOOL)isMenuAgentBundle {
    NSString *bundleIdentifier = NSBundle.mainBundle.bundleIdentifier ?: @"";
    if ([bundleIdentifier isEqualToString:LSMenuAgentBundleIdentifier]) {
        return YES;
    }

    id uiElement = [NSBundle.mainBundle objectForInfoDictionaryKey:@"LSUIElement"];
    return [uiElement respondsToSelector:@selector(boolValue)] ? [uiElement boolValue] : NO;
}

+ (nullable NSURL *)embeddedMenuAgentAppURL {
    NSURL *bundleURL = NSBundle.mainBundle.bundleURL;
    if (!bundleURL) {
        return nil;
    }

    NSURL *contentsURL = [bundleURL URLByAppendingPathComponent:@"Contents" isDirectory:YES];
    NSURL *libraryURL = [contentsURL URLByAppendingPathComponent:@"Library" isDirectory:YES];
    NSURL *loginItemsURL = [libraryURL URLByAppendingPathComponent:@"LoginItems" isDirectory:YES];
    return [loginItemsURL URLByAppendingPathComponent:@"LlamaSitterMenu.app" isDirectory:YES];
}

+ (nullable NSURL *)dashboardAppURL {
    NSURL *bundleURL = NSBundle.mainBundle.bundleURL;
    if (!bundleURL) {
        return nil;
    }

    if (![self isMenuAgentBundle]) {
        return bundleURL;
    }

    NSURL *url = bundleURL;
    for (NSInteger index = 0; index < 4; index += 1) {
        url = [url URLByDeletingLastPathComponent];
    }
    return url;
}

+ (BOOL)isMenuAgentRunning {
    return [NSRunningApplication runningApplicationsWithBundleIdentifier:LSMenuAgentBundleIdentifier].count > 0;
}

+ (NSArray<NSRunningApplication *> *)runningDashboardApplications {
    return [NSRunningApplication runningApplicationsWithBundleIdentifier:LSDashboardBundleIdentifier];
}

@end
