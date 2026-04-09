#import "LSMainWindowController.h"

#import <WebKit/WebKit.h>

@interface LSMainWindowController ()

@property (nonatomic, strong) NSVisualEffectView *loadingView;
@property (nonatomic, strong) NSProgressIndicator *loadingIndicator;
@property (nonatomic, strong) NSTextField *loadingTitleLabel;
@property (nonatomic, strong) NSTextField *loadingMessageLabel;

@property (nonatomic, strong) NSVisualEffectView *errorView;
@property (nonatomic, strong) NSTextField *errorTitleLabel;
@property (nonatomic, strong) NSTextField *errorMessageLabel;
@property (nonatomic, strong) NSButton *retryButton;

@property (nonatomic, strong) WKWebView *webView;

@end

@implementation LSMainWindowController

- (instancetype)init {
    NSWindow *window = [[NSWindow alloc] initWithContentRect:NSMakeRect(0, 0, 1220, 860)
                                                   styleMask:(NSWindowStyleMaskTitled |
                                                              NSWindowStyleMaskClosable |
                                                              NSWindowStyleMaskMiniaturizable |
                                                              NSWindowStyleMaskResizable)
                                                     backing:NSBackingStoreBuffered
                                                       defer:NO];
    [window setTitle:@"LlamaSitter"];
    [window center];
    [window setReleasedWhenClosed:NO];

    self = [super initWithWindow:window];
    if (!self) {
        return nil;
    }

    self.loadingView = [[NSVisualEffectView alloc] initWithFrame:NSZeroRect];
    self.loadingIndicator = [[NSProgressIndicator alloc] initWithFrame:NSZeroRect];
    self.loadingTitleLabel = [NSTextField labelWithString:@"Starting LlamaSitter"];
    self.loadingMessageLabel = [self wrappingLabelWithString:@"Preparing the local metrics service."];

    self.errorView = [[NSVisualEffectView alloc] initWithFrame:NSZeroRect];
    self.errorTitleLabel = [NSTextField labelWithString:@"Unable to load metrics"];
    self.errorMessageLabel = [self wrappingLabelWithString:@""];
    self.retryButton = [NSButton buttonWithTitle:@"Retry" target:self action:@selector(retryAction:)];

    self.webView = [[WKWebView alloc] initWithFrame:NSZeroRect];

    [self setupContent];
    return self;
}

- (void)showLoadingWithTitle:(NSString *)title message:(NSString *)message {
    self.loadingTitleLabel.stringValue = title;
    self.loadingMessageLabel.stringValue = message;
    [self.loadingIndicator startAnimation:nil];
    self.loadingView.hidden = NO;
    self.errorView.hidden = YES;
    self.webView.hidden = YES;
}

- (void)showErrorMessage:(NSString *)message {
    self.errorMessageLabel.stringValue = message;
    [self.loadingIndicator stopAnimation:nil];
    self.loadingView.hidden = YES;
    self.errorView.hidden = NO;
    self.webView.hidden = YES;
}

- (void)showDashboardURL:(NSURL *)url {
    NSURLRequest *request = [NSURLRequest requestWithURL:url];
    [self.webView loadRequest:request];
    [self.loadingIndicator stopAnimation:nil];
    self.loadingView.hidden = YES;
    self.errorView.hidden = YES;
    self.webView.hidden = NO;
}

- (void)restoreAndFocus {
    [self.window makeKeyAndOrderFront:nil];
    [NSApp activateIgnoringOtherApps:YES];
}

- (void)setupContent {
    NSView *contentView = self.window.contentView;
    if (!contentView) {
        return;
    }

    self.webView.translatesAutoresizingMaskIntoConstraints = NO;
    self.webView.hidden = YES;

    [self configureLoadingView];
    [self configureErrorView];

    [contentView addSubview:self.webView];
    [contentView addSubview:self.loadingView];
    [contentView addSubview:self.errorView];

    for (NSView *view in @[self.webView, self.loadingView, self.errorView]) {
        [NSLayoutConstraint activateConstraints:@[
            [view.leadingAnchor constraintEqualToAnchor:contentView.leadingAnchor],
            [view.trailingAnchor constraintEqualToAnchor:contentView.trailingAnchor],
            [view.topAnchor constraintEqualToAnchor:contentView.topAnchor],
            [view.bottomAnchor constraintEqualToAnchor:contentView.bottomAnchor],
        ]];
    }
}

- (void)configureLoadingView {
    self.loadingView.translatesAutoresizingMaskIntoConstraints = NO;
    self.loadingView.material = NSVisualEffectMaterialSidebar;
    self.loadingView.blendingMode = NSVisualEffectBlendingModeBehindWindow;
    self.loadingView.state = NSVisualEffectStateActive;

    self.loadingTitleLabel.font = [NSFont systemFontOfSize:28 weight:NSFontWeightSemibold];
    self.loadingMessageLabel.font = [NSFont systemFontOfSize:14];
    self.loadingMessageLabel.textColor = [NSColor secondaryLabelColor];
    self.loadingMessageLabel.alignment = NSTextAlignmentCenter;

    self.loadingIndicator.controlSize = NSControlSizeRegular;
    self.loadingIndicator.style = NSProgressIndicatorStyleSpinning;
    self.loadingIndicator.translatesAutoresizingMaskIntoConstraints = NO;

    NSStackView *stack = [NSStackView stackViewWithViews:@[
        self.loadingIndicator,
        self.loadingTitleLabel,
        self.loadingMessageLabel,
    ]];
    stack.orientation = NSUserInterfaceLayoutOrientationVertical;
    stack.alignment = NSLayoutAttributeCenterX;
    stack.spacing = 16;
    stack.translatesAutoresizingMaskIntoConstraints = NO;

    [self.loadingView addSubview:stack];
    [NSLayoutConstraint activateConstraints:@[
        [stack.centerXAnchor constraintEqualToAnchor:self.loadingView.centerXAnchor],
        [stack.centerYAnchor constraintEqualToAnchor:self.loadingView.centerYAnchor],
        [stack.widthAnchor constraintLessThanOrEqualToConstant:480],
    ]];
}

- (void)configureErrorView {
    self.errorView.translatesAutoresizingMaskIntoConstraints = NO;
    self.errorView.material = NSVisualEffectMaterialContentBackground;
    self.errorView.blendingMode = NSVisualEffectBlendingModeBehindWindow;
    self.errorView.state = NSVisualEffectStateActive;
    self.errorView.hidden = YES;

    self.errorTitleLabel.font = [NSFont systemFontOfSize:28 weight:NSFontWeightSemibold];
    self.errorMessageLabel.font = [NSFont systemFontOfSize:13];
    self.errorMessageLabel.textColor = [NSColor secondaryLabelColor];
    self.errorMessageLabel.alignment = NSTextAlignmentCenter;

    self.retryButton.bezelStyle = NSBezelStyleRounded;

    NSStackView *stack = [NSStackView stackViewWithViews:@[
        self.errorTitleLabel,
        self.errorMessageLabel,
        self.retryButton,
    ]];
    stack.orientation = NSUserInterfaceLayoutOrientationVertical;
    stack.alignment = NSLayoutAttributeCenterX;
    stack.spacing = 18;
    stack.translatesAutoresizingMaskIntoConstraints = NO;

    [self.errorView addSubview:stack];
    [NSLayoutConstraint activateConstraints:@[
        [stack.centerXAnchor constraintEqualToAnchor:self.errorView.centerXAnchor],
        [stack.centerYAnchor constraintEqualToAnchor:self.errorView.centerYAnchor],
        [stack.widthAnchor constraintLessThanOrEqualToConstant:620],
    ]];
}

- (NSTextField *)wrappingLabelWithString:(NSString *)string {
    NSTextField *label = [NSTextField labelWithString:string];
    label.translatesAutoresizingMaskIntoConstraints = NO;
    label.alignment = NSTextAlignmentCenter;
    label.lineBreakMode = NSLineBreakByWordWrapping;
    label.maximumNumberOfLines = 0;
    label.editable = NO;
    label.selectable = NO;
    label.bezeled = NO;
    label.drawsBackground = NO;
    return label;
}

- (void)retryAction:(__unused id)sender {
    if (self.onRetry) {
        self.onRetry();
    }
}

@end
