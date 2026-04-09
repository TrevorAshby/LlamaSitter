#import <Foundation/Foundation.h>

#import "LSServiceController.h"

NS_ASSUME_NONNULL_BEGIN

@interface LSOverviewSnapshot : NSObject

@property (nonatomic, readonly) LSServiceStatus serviceStatus;
@property (nonatomic, readonly, copy) NSString *errorMessage;
@property (nonatomic, readonly, strong) NSDate *lastRefreshAt;
@property (nonatomic, readonly) NSInteger requestCount;
@property (nonatomic, readonly) NSInteger totalTokens;
@property (nonatomic, readonly) double averageRequestDurationMs;
@property (nonatomic, readonly, copy) NSString *topModel;
@property (nonatomic, readonly, copy) NSString *topClientInstance;
@property (nonatomic, readonly, copy) NSString *activityTitle;
@property (nonatomic, readonly, copy) NSString *activityDetail;

- (instancetype)initWithServiceStatus:(LSServiceStatus)serviceStatus
                         errorMessage:(NSString *)errorMessage
                        lastRefreshAt:(NSDate *)lastRefreshAt
                         requestCount:(NSInteger)requestCount
                          totalTokens:(NSInteger)totalTokens
             averageRequestDurationMs:(double)averageRequestDurationMs
                             topModel:(NSString *)topModel
                    topClientInstance:(NSString *)topClientInstance
                        activityTitle:(NSString *)activityTitle
                       activityDetail:(NSString *)activityDetail NS_DESIGNATED_INITIALIZER;
- (instancetype)init NS_UNAVAILABLE;

@end

NS_ASSUME_NONNULL_END
