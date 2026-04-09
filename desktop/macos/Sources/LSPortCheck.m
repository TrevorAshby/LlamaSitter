#import "LSPortCheck.h"

#import <arpa/inet.h>
#import <netinet/in.h>
#import <sys/socket.h>
#import <unistd.h>

@implementation LSPortCheck

+ (NSArray<NSString *> *)unavailableAddressesInAddresses:(NSArray<NSString *> *)addresses {
    NSMutableArray<NSString *> *unavailable = [NSMutableArray array];
    for (NSString *address in addresses) {
        if (![self canBindAddress:address]) {
            [unavailable addObject:address];
        }
    }
    return unavailable;
}

+ (BOOL)canBindAddress:(NSString *)address {
    NSString *host = nil;
    uint16_t port = 0;
    if (![self parseAddress:address host:&host port:&port]) {
        return NO;
    }

    int socketFD = socket(AF_INET, SOCK_STREAM, 0);
    if (socketFD < 0) {
        return NO;
    }

    int reuse = 1;
    (void)setsockopt(socketFD, SOL_SOCKET, SO_REUSEADDR, &reuse, sizeof(reuse));

    struct sockaddr_in sockAddr;
    memset(&sockAddr, 0, sizeof(sockAddr));
    sockAddr.sin_len = sizeof(sockAddr);
    sockAddr.sin_family = AF_INET;
    sockAddr.sin_port = htons(port);

    if (inet_pton(AF_INET, host.UTF8String, &sockAddr.sin_addr) != 1) {
        close(socketFD);
        return NO;
    }

    int bindResult = bind(socketFD, (const struct sockaddr *)&sockAddr, sizeof(sockAddr));
    close(socketFD);
    return bindResult == 0;
}

+ (BOOL)parseAddress:(NSString *)address host:(NSString * _Nullable * _Nullable)host port:(uint16_t *)port {
    NSArray<NSString *> *parts = [address componentsSeparatedByString:@":"];
    if (parts.count != 2 || parts[0].length == 0) {
        return NO;
    }

    NSInteger parsedPort = [parts[1] integerValue];
    if (parsedPort <= 0 || parsedPort > 65535) {
        return NO;
    }

    if (host) {
        *host = parts[0];
    }
    if (port) {
        *port = (uint16_t)parsedPort;
    }
    return YES;
}

@end
