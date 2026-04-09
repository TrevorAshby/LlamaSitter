#import <Cocoa/Cocoa.h>

#import "LSAppDelegate.h"
#import "LSBundleLocator.h"
#import "LSRuntimeConfig.h"

int main(int argc, const char *argv[]) {
    @autoreleasepool {
        (void)argc;
        (void)argv;

        [LSRuntimeConfig redirectApplicationConsoleToLogWithError:nil];

        NSApplication *application = [NSApplication sharedApplication];
        if (![LSBundleLocator isMenuAgentBundle]) {
            [application setActivationPolicy:NSApplicationActivationPolicyRegular];
        }
        LSAppDelegate *delegate = [[LSAppDelegate alloc] init];
        application.delegate = delegate;
        [application run];
    }

    return EXIT_SUCCESS;
}
