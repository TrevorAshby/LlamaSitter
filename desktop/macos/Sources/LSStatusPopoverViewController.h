#import <Cocoa/Cocoa.h>

#import "LSOverviewSnapshot.h"

NS_ASSUME_NONNULL_BEGIN

@interface LSStatusPopoverViewController : NSViewController

@property (nonatomic, copy, nullable) dispatch_block_t onOpenDashboard;
@property (nonatomic, copy, nullable) dispatch_block_t onRetry;
@property (nonatomic, copy, nullable) dispatch_block_t onQuit;

- (void)updateLocalStatus:(LSServiceStatus)status message:(nullable NSString *)message;
- (void)applyOverviewSnapshot:(nullable LSOverviewSnapshot *)snapshot;

@end

NS_ASSUME_NONNULL_END
