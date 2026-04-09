#import <Foundation/Foundation.h>

NS_ASSUME_NONNULL_BEGIN

@interface LSPortCheck : NSObject

+ (NSArray<NSString *> *)unavailableAddressesInAddresses:(NSArray<NSString *> *)addresses;

@end

NS_ASSUME_NONNULL_END
