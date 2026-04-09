#import <AppKit/AppKit.h>

NS_ASSUME_NONNULL_BEGIN

FOUNDATION_EXPORT NSString *const LSDashboardBundleIdentifier;
FOUNDATION_EXPORT NSString *const LSMenuAgentBundleIdentifier;

@interface LSBundleLocator : NSObject

+ (BOOL)isMenuAgentBundle;
+ (nullable NSURL *)embeddedMenuAgentAppURL;
+ (nullable NSURL *)dashboardAppURL;
+ (BOOL)isMenuAgentRunning;
+ (NSArray<NSRunningApplication *> *)runningDashboardApplications;

@end

NS_ASSUME_NONNULL_END
