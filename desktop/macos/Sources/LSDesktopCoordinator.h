#import <Cocoa/Cocoa.h>

NS_ASSUME_NONNULL_BEGIN

@interface LSDesktopCoordinator : NSObject

- (void)start;
- (void)prepareForTermination;

@end

NS_ASSUME_NONNULL_END
