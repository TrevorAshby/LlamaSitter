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
    dispatch_group_t group = dispatch_group_create();

    __block NSError *readyError = nil;
    __block NSError *summaryError = nil;
    __block NSDictionary *summary = nil;
    __block NSDictionary *sessions = nil;
    __block NSDictionary *requests = nil;

    dispatch_group_enter(group);
    [self fetchJSONForPath:@"readyz" completion:^(__unused id json, NSError *error) {
        readyError = error;
        dispatch_group_leave(group);
    }];

    dispatch_group_enter(group);
    [self fetchJSONForPath:@"api/usage/summary" completion:^(id json, NSError *error) {
        if (error) {
            summaryError = error;
        } else if ([json isKindOfClass:[NSDictionary class]]) {
            summary = json;
        } else {
            summaryError = LSOverviewFetcherError(@"Unexpected usage summary response.");
        }
        dispatch_group_leave(group);
    }];

    dispatch_group_enter(group);
    [self fetchJSONForPath:@"api/sessions?limit=5" completion:^(id json, __unused NSError *error) {
        if ([json isKindOfClass:[NSDictionary class]]) {
            sessions = json;
        }
        dispatch_group_leave(group);
    }];

    dispatch_group_enter(group);
    [self fetchJSONForPath:@"api/requests?limit=5" completion:^(id json, __unused NSError *error) {
        if ([json isKindOfClass:[NSDictionary class]]) {
            requests = json;
        }
        dispatch_group_leave(group);
    }];

    dispatch_group_notify(group, dispatch_get_main_queue(), ^{
        if (readyError || summaryError) {
            completion(nil, readyError ?: summaryError);
            return;
        }

        NSDictionary *topModel = [self firstDictionaryInArray:summary[@"by_model"]];
        NSDictionary *topInstance = [self firstDictionaryInArray:summary[@"by_client_instance"]];
        NSDictionary *session = [self firstDictionaryInArray:sessions[@"items"]];
        NSDictionary *request = [self firstDictionaryInArray:requests[@"items"]];

        NSString *activityTitle = @"No activity yet";
        NSString *activityDetail = @"Requests and sessions will appear here once the proxy captures traffic.";

        if (session.count > 0) {
            NSString *sessionID = [self stringValue:session[@"session_id"] fallback:@"Recent session"];
            NSInteger requestCount = [self integerValue:session[@"request_count"]];
            NSInteger totalTokens = [self integerValue:session[@"total_tokens"]];
            NSString *agentName = [self stringValue:session[@"agent_name"] fallback:@""];

            activityTitle = [NSString stringWithFormat:@"Session %@", sessionID];
            if (agentName.length > 0) {
                activityDetail = [NSString stringWithFormat:@"%ld requests • %ld tokens • %@",
                                  (long)requestCount,
                                  (long)totalTokens,
                                  agentName];
            } else {
                activityDetail = [NSString stringWithFormat:@"%ld requests • %ld tokens",
                                  (long)requestCount,
                                  (long)totalTokens];
            }
        } else if (request.count > 0) {
            NSString *model = [self stringValue:request[@"model"] fallback:@"Recent request"];
            NSInteger totalTokens = [self integerValue:request[@"total_tokens"]];
            NSInteger statusCode = [self integerValue:request[@"http_status"]];

            activityTitle = model;
            activityDetail = [NSString stringWithFormat:@"HTTP %ld • %ld tokens",
                              (long)statusCode,
                              (long)totalTokens];
        }

        LSOverviewSnapshot *snapshot = [[LSOverviewSnapshot alloc] initWithServiceStatus:serviceStatus
                                                                            errorMessage:errorMessage ?: @""
                                                                           lastRefreshAt:[NSDate date]
                                                                            requestCount:[self integerValue:summary[@"request_count"]]
                                                                             totalTokens:[self integerValue:summary[@"total_tokens"]]
                                                                averageRequestDurationMs:[self doubleValue:summary[@"avg_request_duration_ms"]]
                                                                                topModel:[self stringValue:topModel[@"key"] fallback:@"No data yet"]
                                                                       topClientInstance:[self stringValue:topInstance[@"key"] fallback:@"No data yet"]
                                                                           activityTitle:activityTitle
                                                                          activityDetail:activityDetail];
        completion(snapshot, nil);
    });
}

- (void)fetchJSONForPath:(NSString *)path completion:(void (^)(id _Nullable json, NSError * _Nullable error))completion {
    NSURL *url = [self.baseURL URLByAppendingPathComponent:path];
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

- (NSDictionary *)firstDictionaryInArray:(id)value {
    if (![value isKindOfClass:[NSArray class]]) {
        return @{};
    }

    NSArray *array = value;
    if (array.count == 0 || ![array.firstObject isKindOfClass:[NSDictionary class]]) {
        return @{};
    }

    return array.firstObject;
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
