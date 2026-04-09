#import <Foundation/Foundation.h>

NS_ASSUME_NONNULL_BEGIN

@interface LSRuntimeConfig : NSObject

@property (class, nonatomic, readonly, copy) NSString *defaultProxyListenAddr;
@property (class, nonatomic, readonly, copy) NSString *defaultUIListenAddr;
@property (class, nonatomic, readonly, copy) NSString *defaultUpstreamURL;

@property (nonatomic, readonly, strong) NSURL *applicationSupportDirectory;
@property (nonatomic, readonly, strong) NSURL *logsDirectory;
@property (nonatomic, readonly, strong) NSURL *configURL;
@property (nonatomic, readonly, strong) NSURL *databaseURL;
@property (nonatomic, readonly, strong) NSURL *appLogURL;
@property (nonatomic, readonly, strong) NSURL *stdoutLogURL;
@property (nonatomic, readonly, strong) NSURL *backendExecutableURL;
@property (nonatomic, readonly, copy) NSString *proxyListenAddr;
@property (nonatomic, readonly, copy) NSString *uiListenAddr;
@property (nonatomic, readonly, strong) NSURL *uiBaseURL;
@property (nonatomic, readonly, strong) NSURL *readyURL;

- (nullable instancetype)initWithError:(NSError **)error NS_DESIGNATED_INITIALIZER;
- (instancetype)init NS_UNAVAILABLE;

- (nullable NSFileHandle *)openAppLogHandleWithError:(NSError **)error;
- (nullable NSFileHandle *)openCombinedLogHandleWithError:(NSError **)error;

+ (nullable NSURL *)appLogURLWithError:(NSError **)error;
+ (BOOL)redirectApplicationConsoleToLogWithError:(NSError **)error;

@end

NS_ASSUME_NONNULL_END
