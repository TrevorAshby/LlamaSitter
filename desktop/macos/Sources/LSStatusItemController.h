#import <Cocoa/Cocoa.h>

#import "LSOverviewSnapshot.h"

NS_ASSUME_NONNULL_BEGIN

@interface LSStatusItemController : NSObject

@property (nonatomic, copy, nullable) dispatch_block_t onOpenDashboard;
@property (nonatomic, copy, nullable) dispatch_block_t onRetry;
@property (nonatomic, copy, nullable) dispatch_block_t onQuit;
@property (nonatomic, copy, nullable) dispatch_block_t onRefreshRequested;

- (void)updateLocalStatus:(LSServiceStatus)status message:(nullable NSString *)message;
- (void)applyOverviewSnapshot:(nullable LSOverviewSnapshot *)snapshot;
- (void)dismissPopover;

@end

NS_ASSUME_NONNULL_END
