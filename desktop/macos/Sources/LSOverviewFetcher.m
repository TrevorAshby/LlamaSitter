#import "LSOverviewFetcher.h"

static NSError *LSOverviewFetcherError(NSString *message) {
    return [NSError errorWithDomain:@"LlamaSitterDesktopOverview"
                               code:1
                           userInfo:@{NSLocalizedDescriptionKey: message}];
}

@interface LSOverviewFetcher ()

@property (nonatomic, strong) NSURL *baseURL;
@property (nonatomic, strong) NSURLSession *session;

@end

@implementation LSOverviewFetcher

- (instancetype)initWithBaseURL:(NSURL *)baseURL {
    self = [super init];
    if (!self) {
        return nil;
    }

    _baseURL = baseURL;

    NSURLSessionConfiguration *configuration = [NSURLSessionConfiguration ephemeralSessionConfiguration];
    configuration.timeoutIntervalForRequest = 2.0;
    configuration.timeoutIntervalForResource = 4.0;
    _session = [NSURLSession sessionWithConfiguration:configuration];

    return self;
}

- (void)fetchSnapshotForServiceStatus:(LSServiceStatus)serviceStatus
                         errorMessage:(nullable NSString *)errorMessage
                           completion:(void (^)(LSOverviewSnapshot * _Nullable snapshot, NSError * _Nullable error))completion {
    [self fetchJSONForPath:@"api/desktop/overview" completion:^(id json, NSError *error) {
        dispatch_async(dispatch_get_main_queue(), ^{
            if (error) {
                completion(nil, error);
                return;
            }

            if (![json isKindOfClass:[NSDictionary class]]) {
                completion(nil, LSOverviewFetcherError(@"Unexpected desktop overview response."));
                return;
            }

            NSDictionary *overview = (NSDictionary *)json;
            LSOverviewSnapshot *snapshot = [[LSOverviewSnapshot alloc] initWithServiceStatus:serviceStatus
                                                                                errorMessage:errorMessage ?: @""
                                                                               lastRefreshAt:[self dateValue:overview[@"last_refresh_at"]]
                                                                                requestCount:[self integerValue:overview[@"request_count"]]
                                                                                 totalTokens:[self integerValue:overview[@"total_tokens"]]
                                                                    averageRequestDurationMs:[self doubleValue:overview[@"average_request_duration_ms"]]
                                                                                    topModel:[self stringValue:overview[@"top_model"] fallback:@"No data yet"]
                                                                           topClientInstance:[self stringValue:overview[@"top_client_instance"] fallback:@"No data yet"]
                                                                               activityTitle:[self stringValue:overview[@"activity_title"] fallback:@"No activity yet"]
                                                                              activityDetail:[self stringValue:overview[@"activity_detail"] fallback:@"Requests and sessions will appear here once the proxy captures traffic."]];
            completion(snapshot, nil);
        });
    }];
}

- (NSDate *)dateValue:(id)value {
    if ([value isKindOfClass:[NSDate class]]) {
        return value;
    }

    if ([value isKindOfClass:[NSString class]]) {
        static NSISO8601DateFormatter *formatter;
        static dispatch_once_t onceToken;
        dispatch_once(&onceToken, ^{
            formatter = [[NSISO8601DateFormatter alloc] init];
            formatter.formatOptions = NSISO8601DateFormatWithInternetDateTime | NSISO8601DateFormatWithFractionalSeconds;
        });

        NSDate *date = [formatter dateFromString:value];
        if (date) {
            return date;
        }

        static NSISO8601DateFormatter *fallbackFormatter;
        static dispatch_once_t fallbackOnceToken;
        dispatch_once(&fallbackOnceToken, ^{
            fallbackFormatter = [[NSISO8601DateFormatter alloc] init];
            fallbackFormatter.formatOptions = NSISO8601DateFormatWithInternetDateTime;
        });

        date = [fallbackFormatter dateFromString:value];
        if (date) {
            return date;
        }
    }

    return [NSDate date];
}

- (void)fetchJSONForPath:(NSString *)path completion:(void (^)(id _Nullable json, NSError * _Nullable error))completion {
    NSURL *url = [NSURL URLWithString:path relativeToURL:self.baseURL];
    if (!url) {
        completion(nil, LSOverviewFetcherError(@"Unable to build desktop overview URL."));
        return;
    }

    NSMutableURLRequest *request = [NSMutableURLRequest requestWithURL:url];
    request.timeoutInterval = 2.0;

    NSURLSessionDataTask *task = [self.session dataTaskWithRequest:request
                                                 completionHandler:^(NSData *data,
                                                                     NSURLResponse *response,
                                                                     NSError *error) {
        if (error) {
            completion(nil, error);
            return;
        }

        NSHTTPURLResponse *httpResponse = (NSHTTPURLResponse *)response;
        if (![httpResponse isKindOfClass:[NSHTTPURLResponse class]] || httpResponse.statusCode < 200 || httpResponse.statusCode >= 300) {
            NSString *message = [NSString stringWithFormat:@"Request to %@ failed.", path];
            completion(nil, LSOverviewFetcherError(message));
            return;
        }

        if (data.length == 0) {
            completion(@{}, nil);
            return;
        }

        NSError *jsonError = nil;
        id json = [NSJSONSerialization JSONObjectWithData:data options:0 error:&jsonError];
        completion(json, jsonError);
    }];
    [task resume];
}

- (NSString *)stringValue:(id)value fallback:(NSString *)fallback {
    if ([value isKindOfClass:[NSString class]] && [value length] > 0) {
        return value;
    }
    return fallback;
}

- (NSInteger)integerValue:(id)value {
    if ([value isKindOfClass:[NSNumber class]]) {
        return [value integerValue];
    }
    if ([value isKindOfClass:[NSString class]]) {
        return [value integerValue];
    }
    return 0;
}

- (double)doubleValue:(id)value {
    if ([value isKindOfClass:[NSNumber class]]) {
        return [value doubleValue];
    }
    if ([value isKindOfClass:[NSString class]]) {
        return [value doubleValue];
    }
    return 0;
}

@end
