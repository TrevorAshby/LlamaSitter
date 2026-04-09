#import "LSOverviewSnapshot.h"

@implementation LSOverviewSnapshot

- (instancetype)initWithServiceStatus:(LSServiceStatus)serviceStatus
                         errorMessage:(NSString *)errorMessage
                        lastRefreshAt:(NSDate *)lastRefreshAt
                         requestCount:(NSInteger)requestCount
                          totalTokens:(NSInteger)totalTokens
             averageRequestDurationMs:(double)averageRequestDurationMs
                             topModel:(NSString *)topModel
                    topClientInstance:(NSString *)topClientInstance
                        activityTitle:(NSString *)activityTitle
                       activityDetail:(NSString *)activityDetail {
    self = [super init];
    if (!self) {
        return nil;
    }

    _serviceStatus = serviceStatus;
    _errorMessage = [errorMessage copy];
    _lastRefreshAt = lastRefreshAt;
    _requestCount = requestCount;
    _totalTokens = totalTokens;
    _averageRequestDurationMs = averageRequestDurationMs;
    _topModel = [topModel copy];
    _topClientInstance = [topClientInstance copy];
    _activityTitle = [activityTitle copy];
    _activityDetail = [activityDetail copy];

    return self;
}

@end
