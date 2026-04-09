#import "LSRuntimeConfig.h"

#import <fcntl.h>
#import <unistd.h>

static NSString *const LSDefaultProxyListenAddr = @"127.0.0.1:11435";
static NSString *const LSDefaultUIListenAddr = @"127.0.0.1:11438";
static NSString *const LSDefaultUpstreamURL = @"http://127.0.0.1:11434";

static NSError *LSRuntimeConfigError(NSString *message) {
    return [NSError errorWithDomain:@"LlamaSitterDesktop"
                               code:1
                           userInfo:@{NSLocalizedDescriptionKey: message}];
}

@interface LSRuntimeConfig ()

@property (nonatomic, readwrite, strong) NSURL *applicationSupportDirectory;
@property (nonatomic, readwrite, strong) NSURL *logsDirectory;
@property (nonatomic, readwrite, strong) NSURL *configURL;
@property (nonatomic, readwrite, strong) NSURL *databaseURL;
@property (nonatomic, readwrite, strong) NSURL *appLogURL;
@property (nonatomic, readwrite, strong) NSURL *stdoutLogURL;
@property (nonatomic, readwrite, strong) NSURL *backendExecutableURL;
@property (nonatomic, readwrite, copy) NSString *proxyListenAddr;
@property (nonatomic, readwrite, copy) NSString *uiListenAddr;
@property (nonatomic, readwrite, strong) NSURL *uiBaseURL;
@property (nonatomic, readwrite, strong) NSURL *readyURL;

@end

@implementation LSRuntimeConfig

+ (NSString *)defaultProxyListenAddr {
    return LSDefaultProxyListenAddr;
}

+ (NSString *)defaultUIListenAddr {
    return LSDefaultUIListenAddr;
}

+ (NSString *)defaultUpstreamURL {
    return LSDefaultUpstreamURL;
}

- (nullable instancetype)initWithError:(NSError **)error {
    self = [super init];
    if (!self) {
        return nil;
    }

    NSFileManager *fileManager = [NSFileManager defaultManager];

    NSURL *appSupportRoot = [fileManager URLForDirectory:NSApplicationSupportDirectory
                                                inDomain:NSUserDomainMask
                                       appropriateForURL:nil
                                                  create:YES
                                                   error:error];
    if (!appSupportRoot) {
        return nil;
    }

    NSURL *libraryRoot = [fileManager URLForDirectory:NSLibraryDirectory
                                             inDomain:NSUserDomainMask
                                    appropriateForURL:nil
                                               create:YES
                                                error:error];
    if (!libraryRoot) {
        return nil;
    }

    NSURL *logsDirectory = [[self class] logsDirectoryWithLibraryRoot:libraryRoot];

    NSURL *resourceURL = [NSBundle mainBundle].resourceURL;
    if (!resourceURL) {
        if (error) {
            *error = LSRuntimeConfigError(@"Unable to resolve bundle resources.");
        }
        return nil;
    }

    self.applicationSupportDirectory = [appSupportRoot URLByAppendingPathComponent:@"LlamaSitter" isDirectory:YES];
    self.logsDirectory = logsDirectory;
    self.configURL = [self.applicationSupportDirectory URLByAppendingPathComponent:@"llamasitter.yaml"];
    self.databaseURL = [self.applicationSupportDirectory URLByAppendingPathComponent:@"llamasitter.db"];
    self.appLogURL = [self.logsDirectory URLByAppendingPathComponent:@"app.log"];
    self.stdoutLogURL = [self.logsDirectory URLByAppendingPathComponent:@"backend.log"];
    self.backendExecutableURL = [resourceURL URLByAppendingPathComponent:@"llamasitter-backend"];

    if (![fileManager createDirectoryAtURL:self.applicationSupportDirectory
               withIntermediateDirectories:YES
                                attributes:nil
                                     error:error]) {
        return nil;
    }

    if (![fileManager createDirectoryAtURL:self.logsDirectory
               withIntermediateDirectories:YES
                                attributes:nil
                                     error:error]) {
        return nil;
    }

    if (![fileManager fileExistsAtPath:self.configURL.path]) {
        if (![[self class] writeDefaultConfigToURL:self.configURL
                                        databaseURL:self.databaseURL
                                              error:error]) {
            return nil;
        }
    }

    NSDictionary<NSString *, NSString *> *parsed = [[self class] parseConfigAtURL:self.configURL error:error];
    if (!parsed) {
        return nil;
    }

    self.proxyListenAddr = parsed[@"proxy"];
    self.uiListenAddr = parsed[@"ui"];

    NSURL *uiBaseURL = [[self class] baseURLForListenAddr:self.uiListenAddr error:error];
    if (!uiBaseURL) {
        return nil;
    }

    self.uiBaseURL = uiBaseURL;
    self.readyURL = [uiBaseURL URLByAppendingPathComponent:@"readyz"];

    return self;
}

- (nullable NSFileHandle *)openAppLogHandleWithError:(NSError **)error {
    return [[self class] openLogHandleAtURL:self.appLogURL error:error];
}

- (nullable NSFileHandle *)openCombinedLogHandleWithError:(NSError **)error {
    return [[self class] openLogHandleAtURL:self.stdoutLogURL error:error];
}

+ (BOOL)writeDefaultConfigToURL:(NSURL *)url
                    databaseURL:(NSURL *)databaseURL
                          error:(NSError **)error {
    NSString *yaml = [NSString stringWithFormat:
                      @"listeners:\n"
                      @"  - name: default\n"
                      @"    listen_addr: \"%@\"\n"
                      @"    upstream_url: \"%@\"\n"
                      @"    default_tags:\n"
                      @"      client_type: \"dock-app\"\n"
                      @"      client_instance: \"macos\"\n"
                      @"\n"
                      @"storage:\n"
                      @"  sqlite_path: \"%@\"\n"
                      @"\n"
                      @"privacy:\n"
                      @"  persist_bodies: false\n"
                      @"  redact_headers:\n"
                      @"    - authorization\n"
                      @"    - proxy-authorization\n"
                      @"  redact_json_fields:\n"
                      @"    - prompt\n"
                      @"    - messages\n"
                      @"\n"
                      @"ui:\n"
                      @"  enabled: true\n"
                      @"  listen_addr: \"%@\"\n",
                      self.defaultProxyListenAddr,
                      self.defaultUpstreamURL,
                      [self escapeYAMLValue:databaseURL.path],
                      self.defaultUIListenAddr];

    return [yaml writeToURL:url atomically:YES encoding:NSUTF8StringEncoding error:error];
}

+ (nullable NSDictionary<NSString *, NSString *> *)parseConfigAtURL:(NSURL *)url
                                                              error:(NSError **)error {
    NSString *contents = [NSString stringWithContentsOfURL:url
                                                  encoding:NSUTF8StringEncoding
                                                     error:error];
    if (!contents) {
        return nil;
    }

    NSString *section = nil;
    NSString *proxyListenAddr = nil;
    NSString *uiListenAddr = nil;

    NSArray<NSString *> *lines = [contents componentsSeparatedByCharactersInSet:[NSCharacterSet newlineCharacterSet]];
    for (NSString *rawLine in lines) {
        NSString *trimmed = [rawLine stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
        if (trimmed.length == 0 || [trimmed hasPrefix:@"#"]) {
            continue;
        }

        if (![rawLine hasPrefix:@" "] && [trimmed hasSuffix:@":"]) {
            section = [trimmed substringToIndex:trimmed.length - 1];
            continue;
        }

        if ([trimmed hasPrefix:@"listen_addr:"]) {
            NSString *value = [self parseScalarFromLine:trimmed key:@"listen_addr"];
            if ([section isEqualToString:@"listeners"]) {
                if (!proxyListenAddr) {
                    proxyListenAddr = value;
                }
            } else if ([section isEqualToString:@"ui"]) {
                uiListenAddr = value;
            }
        }
    }

    if (proxyListenAddr.length == 0) {
        if (error) {
            *error = LSRuntimeConfigError([NSString stringWithFormat:@"Unable to determine proxy listen address from %@.", url.path]);
        }
        return nil;
    }

    if (uiListenAddr.length == 0) {
        if (error) {
            *error = LSRuntimeConfigError([NSString stringWithFormat:@"Unable to determine UI listen address from %@.", url.path]);
        }
        return nil;
    }

    return @{
        @"proxy": proxyListenAddr,
        @"ui": uiListenAddr,
    };
}

+ (NSString *)parseScalarFromLine:(NSString *)line key:(NSString *)key {
    NSString *prefix = [NSString stringWithFormat:@"%@:", key];
    NSString *raw = [[line substringFromIndex:prefix.length] stringByTrimmingCharactersInSet:[NSCharacterSet whitespaceCharacterSet]];
    if (raw.length >= 2 && [raw hasPrefix:@"\""] && [raw hasSuffix:@"\""]) {
        return [raw substringWithRange:NSMakeRange(1, raw.length - 2)];
    }
    return raw;
}

+ (nullable NSURL *)baseURLForListenAddr:(NSString *)listenAddr error:(NSError **)error {
    NSArray<NSString *> *parts = [listenAddr componentsSeparatedByString:@":"];
    if (parts.count != 2) {
        if (error) {
            *error = LSRuntimeConfigError([NSString stringWithFormat:@"Invalid listen address: %@", listenAddr]);
        }
        return nil;
    }

    NSInteger port = [parts[1] integerValue];
    if (port <= 0 || port > 65535 || parts[0].length == 0) {
        if (error) {
            *error = LSRuntimeConfigError([NSString stringWithFormat:@"Invalid listen address: %@", listenAddr]);
        }
        return nil;
    }

    NSURLComponents *components = [[NSURLComponents alloc] init];
    components.scheme = @"http";
    components.host = parts[0];
    components.port = @(port);

    NSURL *url = components.URL;
    if (!url && error) {
        *error = LSRuntimeConfigError([NSString stringWithFormat:@"Unable to build UI URL for %@.", listenAddr]);
    }
    return url;
}

+ (NSString *)escapeYAMLValue:(NSString *)value {
    NSString *escaped = [value stringByReplacingOccurrencesOfString:@"\\" withString:@"\\\\"];
    return [escaped stringByReplacingOccurrencesOfString:@"\"" withString:@"\\\""];
}

+ (nullable NSURL *)appLogURLWithError:(NSError **)error {
    NSFileManager *fileManager = [NSFileManager defaultManager];
    NSURL *libraryRoot = [fileManager URLForDirectory:NSLibraryDirectory
                                             inDomain:NSUserDomainMask
                                    appropriateForURL:nil
                                               create:YES
                                                error:error];
    if (!libraryRoot) {
        return nil;
    }

    NSURL *logsDirectory = [self logsDirectoryWithLibraryRoot:libraryRoot];
    if (![fileManager createDirectoryAtURL:logsDirectory
               withIntermediateDirectories:YES
                                attributes:nil
                                     error:error]) {
        return nil;
    }

    return [logsDirectory URLByAppendingPathComponent:@"app.log"];
}

+ (BOOL)redirectApplicationConsoleToLogWithError:(NSError **)error {
    NSURL *appLogURL = [self appLogURLWithError:error];
    if (!appLogURL) {
        return NO;
    }

    int fileDescriptor = open(appLogURL.fileSystemRepresentation, O_WRONLY | O_CREAT | O_APPEND, 0644);
    if (fileDescriptor < 0) {
        if (error) {
            NSString *message = [NSString stringWithFormat:@"Unable to open app log at %@.", appLogURL.path];
            *error = LSRuntimeConfigError(message);
        }
        return NO;
    }

    dup2(fileDescriptor, STDOUT_FILENO);
    dup2(fileDescriptor, STDERR_FILENO);
    close(fileDescriptor);
    return YES;
}

+ (NSURL *)logsDirectoryWithLibraryRoot:(NSURL *)libraryRoot {
    NSURL *logsRoot = [libraryRoot URLByAppendingPathComponent:@"Logs" isDirectory:YES];
    return [logsRoot URLByAppendingPathComponent:@"LlamaSitter" isDirectory:YES];
}

+ (nullable NSFileHandle *)openLogHandleAtURL:(NSURL *)url error:(NSError **)error {
    NSFileManager *fileManager = [NSFileManager defaultManager];
    if (![fileManager fileExistsAtPath:url.path]) {
        [fileManager createFileAtPath:url.path contents:nil attributes:nil];
    }

    NSFileHandle *handle = [NSFileHandle fileHandleForWritingToURL:url error:error];
    if (!handle) {
        return nil;
    }

    [handle seekToEndOfFile];
    return handle;
}

@end
