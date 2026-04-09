#import "LSStatusPopoverViewController.h"

@interface LSStatusPopoverViewController ()

@property (nonatomic, strong) NSTextField *titleLabel;
@property (nonatomic, strong) NSTextField *statusValueLabel;
@property (nonatomic, strong) NSTextField *lastUpdatedLabel;
@property (nonatomic, strong) NSTextField *errorMessageLabel;

@property (nonatomic, strong) NSTextField *requestsValueLabel;
@property (nonatomic, strong) NSTextField *tokensValueLabel;
@property (nonatomic, strong) NSTextField *durationValueLabel;
@property (nonatomic, strong) NSTextField *topModelValueLabel;
@property (nonatomic, strong) NSTextField *topInstanceValueLabel;
@property (nonatomic, strong) NSTextField *activityTitleLabel;
@property (nonatomic, strong) NSTextField *activityDetailLabel;

@property (nonatomic, strong) NSButton *openDashboardButton;
@property (nonatomic, strong) NSButton *retryButton;
@property (nonatomic, strong) NSButton *quitButton;

@property (nonatomic) LSServiceStatus localStatus;
@property (nonatomic, copy) NSString *localMessage;
@property (nonatomic, strong, nullable) LSOverviewSnapshot *snapshot;

@end

@implementation LSStatusPopoverViewController

- (instancetype)init {
    self = [super initWithNibName:nil bundle:nil];
    if (!self) {
        return nil;
    }

    _localStatus = LSServiceStatusIdle;
    _localMessage = @"";
    self.preferredContentSize = NSMakeSize(320, 310);
    return self;
}

- (void)loadView {
    NSView *rootView = [[NSView alloc] initWithFrame:NSMakeRect(0, 0, self.preferredContentSize.width, self.preferredContentSize.height)];
    rootView.wantsLayer = YES;
    rootView.layer.cornerRadius = 14.0;
    rootView.layer.masksToBounds = YES;
    self.view = rootView;
    [self applyPanelBackgroundColor];

    self.titleLabel = [NSTextField labelWithString:@"LlamaSitter"];
    self.titleLabel.font = [NSFont systemFontOfSize:16 weight:NSFontWeightSemibold];

    self.statusValueLabel = [NSTextField labelWithString:@"Starting"];
    self.statusValueLabel.font = [NSFont systemFontOfSize:13 weight:NSFontWeightSemibold];

    self.lastUpdatedLabel = [NSTextField labelWithString:@"Waiting for the local API…"];
    self.lastUpdatedLabel.font = [NSFont systemFontOfSize:11];
    self.lastUpdatedLabel.textColor = [NSColor secondaryLabelColor];

    self.errorMessageLabel = [self wrappingLabelWithString:@""];
    self.errorMessageLabel.font = [NSFont systemFontOfSize:11];
    self.errorMessageLabel.textColor = [NSColor secondaryLabelColor];

    self.requestsValueLabel = [self metricValueLabel];
    self.tokensValueLabel = [self metricValueLabel];
    self.durationValueLabel = [self metricValueLabel];
    self.topModelValueLabel = [self metricValueLabel];
    self.topInstanceValueLabel = [self metricValueLabel];
    self.activityTitleLabel = [self metricValueLabel];
    self.activityDetailLabel = [self wrappingLabelWithString:@""];
    self.activityDetailLabel.font = [NSFont systemFontOfSize:11];
    self.activityDetailLabel.textColor = [NSColor secondaryLabelColor];

    self.openDashboardButton = [NSButton buttonWithTitle:@"Open Dashboard" target:self action:@selector(openDashboardAction:)];
    self.retryButton = [NSButton buttonWithTitle:@"Retry" target:self action:@selector(retryAction:)];
    self.quitButton = [NSButton buttonWithTitle:@"Quit LlamaSitter" target:self action:@selector(quitAction:)];
    self.quitButton.bezelColor = [NSColor controlAccentColor];

    NSStackView *headerRow = [NSStackView stackViewWithViews:@[self.titleLabel, self.statusValueLabel]];
    headerRow.orientation = NSUserInterfaceLayoutOrientationHorizontal;
    headerRow.alignment = NSLayoutAttributeFirstBaseline;
    headerRow.distribution = NSStackViewDistributionFillProportionally;

    NSStackView *contentStack = [NSStackView stackViewWithViews:@[
        headerRow,
        self.lastUpdatedLabel,
        self.errorMessageLabel,
        [self separator],
        [self metricRowWithLabel:@"Requests" valueLabel:self.requestsValueLabel],
        [self metricRowWithLabel:@"Total Tokens" valueLabel:self.tokensValueLabel],
        [self metricRowWithLabel:@"Average Duration" valueLabel:self.durationValueLabel],
        [self separator],
        [self metricRowWithLabel:@"Top Model" valueLabel:self.topModelValueLabel],
        [self metricRowWithLabel:@"Top Instance" valueLabel:self.topInstanceValueLabel],
        [self metricRowWithLabel:@"Latest Activity" valueLabel:self.activityTitleLabel],
        self.activityDetailLabel,
        [self separator],
        [self actionRow],
    ]];
    contentStack.orientation = NSUserInterfaceLayoutOrientationVertical;
    contentStack.alignment = NSLayoutAttributeLeading;
    contentStack.spacing = 10;
    contentStack.translatesAutoresizingMaskIntoConstraints = NO;

    [rootView addSubview:contentStack];
    [NSLayoutConstraint activateConstraints:@[
        [contentStack.leadingAnchor constraintEqualToAnchor:rootView.leadingAnchor constant:14],
        [contentStack.trailingAnchor constraintEqualToAnchor:rootView.trailingAnchor constant:-14],
        [contentStack.topAnchor constraintEqualToAnchor:rootView.topAnchor constant:14],
        [contentStack.bottomAnchor constraintLessThanOrEqualToAnchor:rootView.bottomAnchor constant:-14],
    ]];

    [self render];
}

- (void)viewDidAppear {
    [super viewDidAppear];
    [self applyPanelBackgroundColor];
}

- (void)updateLocalStatus:(LSServiceStatus)status message:(nullable NSString *)message {
    self.localStatus = status;
    self.localMessage = message ?: @"";
    [self render];
}

- (void)applyOverviewSnapshot:(nullable LSOverviewSnapshot *)snapshot {
    self.snapshot = snapshot;
    [self render];
}

- (void)render {
    NSString *statusText = @"Stopped";
    NSColor *statusColor = [NSColor secondaryLabelColor];
    NSString *message = self.localMessage;

    switch (self.localStatus) {
        case LSServiceStatusStarting:
            statusText = @"Starting";
            statusColor = [NSColor systemOrangeColor];
            if (message.length == 0) {
                message = @"Launching the bundled local service.";
            }
            break;
        case LSServiceStatusReady:
            statusText = @"Ready";
            statusColor = [NSColor systemGreenColor];
            break;
        case LSServiceStatusFailed:
            statusText = @"Error";
            statusColor = [NSColor systemRedColor];
            if (message.length == 0) {
                message = @"The bundled local service is unavailable.";
            }
            break;
        case LSServiceStatusIdle:
        default:
            statusText = @"Stopped";
            statusColor = [NSColor secondaryLabelColor];
            if (message.length == 0) {
                message = @"The bundled local service is not running.";
            }
            break;
    }

    self.statusValueLabel.stringValue = statusText;
    self.statusValueLabel.textColor = statusColor;

    BOOL hasFreshSnapshot = self.snapshot != nil && self.localStatus == LSServiceStatusReady;
    if (hasFreshSnapshot) {
        self.lastUpdatedLabel.stringValue = [NSString stringWithFormat:@"Updated %@", [self timeString:self.snapshot.lastRefreshAt]];
        self.requestsValueLabel.stringValue = [self numberString:self.snapshot.requestCount];
        self.tokensValueLabel.stringValue = [self numberString:self.snapshot.totalTokens];
        self.durationValueLabel.stringValue = [NSString stringWithFormat:@"%@ ms", [self numberString:(NSInteger)llround(self.snapshot.averageRequestDurationMs)]];
        self.topModelValueLabel.stringValue = self.snapshot.topModel;
        self.topInstanceValueLabel.stringValue = self.snapshot.topClientInstance;
        self.activityTitleLabel.stringValue = self.snapshot.activityTitle;
        self.activityDetailLabel.stringValue = self.snapshot.activityDetail;
        self.errorMessageLabel.hidden = YES;
    } else {
        self.lastUpdatedLabel.stringValue = self.localStatus == LSServiceStatusReady ? @"Waiting for the latest metrics…" : @"Metrics will appear once the local API is ready.";
        self.requestsValueLabel.stringValue = @"—";
        self.tokensValueLabel.stringValue = @"—";
        self.durationValueLabel.stringValue = @"—";
        self.topModelValueLabel.stringValue = @"No data yet";
        self.topInstanceValueLabel.stringValue = @"No data yet";
        self.activityTitleLabel.stringValue = @"No activity yet";
        self.activityDetailLabel.stringValue = @"The menu bar overview will populate after the proxy captures traffic.";
        self.errorMessageLabel.stringValue = message;
        self.errorMessageLabel.hidden = message.length == 0;
    }

    self.retryButton.hidden = self.localStatus == LSServiceStatusReady;
    self.retryButton.enabled = self.localStatus == LSServiceStatusFailed || self.localStatus == LSServiceStatusIdle;
}

- (NSTextField *)metricValueLabel {
    NSTextField *label = [NSTextField labelWithString:@"—"];
    label.font = [NSFont systemFontOfSize:12 weight:NSFontWeightSemibold];
    label.lineBreakMode = NSLineBreakByTruncatingTail;
    return label;
}

- (NSTextField *)wrappingLabelWithString:(NSString *)string {
    NSTextField *label = [NSTextField labelWithString:string];
    label.alignment = NSTextAlignmentLeft;
    label.lineBreakMode = NSLineBreakByWordWrapping;
    label.maximumNumberOfLines = 0;
    label.selectable = NO;
    return label;
}

- (NSView *)metricRowWithLabel:(NSString *)label valueLabel:(NSTextField *)valueLabel {
    NSTextField *nameLabel = [NSTextField labelWithString:label];
    nameLabel.font = [NSFont systemFontOfSize:12];
    nameLabel.textColor = [NSColor secondaryLabelColor];

    NSStackView *row = [NSStackView stackViewWithViews:@[nameLabel, valueLabel]];
    row.orientation = NSUserInterfaceLayoutOrientationHorizontal;
    row.alignment = NSLayoutAttributeFirstBaseline;
    row.distribution = NSStackViewDistributionEqualSpacing;
    return row;
}

- (NSView *)actionRow {
    self.openDashboardButton.bezelStyle = NSBezelStyleRounded;
    self.retryButton.bezelStyle = NSBezelStyleRounded;
    self.quitButton.bezelStyle = NSBezelStyleRounded;

    NSStackView *row = [NSStackView stackViewWithViews:@[
        self.openDashboardButton,
        self.retryButton,
        self.quitButton,
    ]];
    row.orientation = NSUserInterfaceLayoutOrientationHorizontal;
    row.alignment = NSLayoutAttributeCenterY;
    row.distribution = NSStackViewDistributionFillEqually;
    row.spacing = 8;
    return row;
}

- (NSView *)separator {
    NSBox *separator = [[NSBox alloc] initWithFrame:NSZeroRect];
    separator.boxType = NSBoxSeparator;
    return separator;
}

- (NSString *)numberString:(NSInteger)value {
    static NSNumberFormatter *formatter;
    static dispatch_once_t onceToken;
    dispatch_once(&onceToken, ^{
        formatter = [[NSNumberFormatter alloc] init];
        formatter.numberStyle = NSNumberFormatterDecimalStyle;
    });
    return [formatter stringFromNumber:@(value)] ?: @"0";
}

- (NSString *)timeString:(NSDate *)date {
    static NSDateFormatter *formatter;
    static dispatch_once_t onceToken;
    dispatch_once(&onceToken, ^{
        formatter = [[NSDateFormatter alloc] init];
        formatter.timeStyle = NSDateFormatterShortStyle;
        formatter.dateStyle = NSDateFormatterNoStyle;
    });
    return [formatter stringFromDate:date];
}

- (void)openDashboardAction:(__unused id)sender {
    if (self.onOpenDashboard) {
        self.onOpenDashboard();
    }
}

- (void)retryAction:(__unused id)sender {
    if (self.onRetry) {
        self.onRetry();
    }
}

- (void)quitAction:(__unused id)sender {
    if (self.onQuit) {
        self.onQuit();
    }
}

- (void)applyPanelBackgroundColor {
    self.view.layer.backgroundColor = [self panelBackgroundColor].CGColor;
}

- (NSColor *)panelBackgroundColor {
    NSString *appearanceName = [self.view.effectiveAppearance bestMatchFromAppearancesWithNames:@[
        NSAppearanceNameDarkAqua,
        NSAppearanceNameAqua,
    ]];

    if ([appearanceName isEqualToString:NSAppearanceNameDarkAqua]) {
        return [NSColor colorWithCalibratedWhite:0.05 alpha:0.995];
    }

    return [NSColor colorWithCalibratedWhite:0.64 alpha:0.995];
}

@end
