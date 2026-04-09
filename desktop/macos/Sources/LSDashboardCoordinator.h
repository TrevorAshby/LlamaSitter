#import <Cocoa/Cocoa.h>

NS_ASSUME_NONNULL_BEGIN

@interface LSDashboardCoordinator : NSObject

- (void)start;
- (BOOL)handleApplicationReopen;
- (void)prepareForTermination;
- (BOOL)shouldTerminateAfterLastWindowClosed;

@end

NS_ASSUME_NONNULL_END
