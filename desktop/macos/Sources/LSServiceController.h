#import <Foundation/Foundation.h>

#import "LSRuntimeConfig.h"

NS_ASSUME_NONNULL_BEGIN

typedef NS_ENUM(NSInteger, LSServiceStatus) {
    LSServiceStatusIdle = 0,
    LSServiceStatusStarting,
    LSServiceStatusReady,
    LSServiceStatusFailed,
};

typedef void (^LSServiceStatusHandler)(LSServiceStatus status, NSURL * _Nullable dashboardURL, NSString * _Nullable message);

@interface LSServiceController : NSObject

@property (nonatomic, readonly) LSServiceStatus status;
@property (nonatomic, copy, nullable) LSServiceStatusHandler onStatusChange;

- (instancetype)initWithRuntime:(LSRuntimeConfig *)runtime NS_DESIGNATED_INITIALIZER;
- (instancetype)init NS_UNAVAILABLE;

- (void)start;
- (void)stop;

@end

NS_ASSUME_NONNULL_END
