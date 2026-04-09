#import <Foundation/Foundation.h>

#import "LSOverviewSnapshot.h"

NS_ASSUME_NONNULL_BEGIN

@interface LSOverviewFetcher : NSObject

- (instancetype)initWithBaseURL:(NSURL *)baseURL NS_DESIGNATED_INITIALIZER;
- (instancetype)init NS_UNAVAILABLE;

- (void)fetchSnapshotForServiceStatus:(LSServiceStatus)serviceStatus
                         errorMessage:(nullable NSString *)errorMessage
                           completion:(void (^)(LSOverviewSnapshot * _Nullable snapshot, NSError * _Nullable error))completion;

@end

NS_ASSUME_NONNULL_END
