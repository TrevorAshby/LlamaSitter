#import <Cocoa/Cocoa.h>

NS_ASSUME_NONNULL_BEGIN

@interface LSMainWindowController : NSWindowController

@property (nonatomic, copy, nullable) dispatch_block_t onRetry;

- (void)showLoadingWithTitle:(NSString *)title message:(NSString *)message;
- (void)showErrorMessage:(NSString *)message;
- (void)showDashboardURL:(NSURL *)url;
- (void)restoreAndFocus;

@end

NS_ASSUME_NONNULL_END
