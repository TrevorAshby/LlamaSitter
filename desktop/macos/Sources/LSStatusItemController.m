#import "LSStatusItemController.h"

#import "LSStatusPanel.h"
#import "LSStatusPopoverViewController.h"

@interface LSStatusItemController ()

@property (nonatomic, strong) NSStatusItem *statusItem;
@property (nonatomic, strong) LSStatusPanel *panel;
@property (nonatomic, strong) LSStatusPopoverViewController *contentController;
@property (nonatomic, strong, nullable) NSTimer *refreshTimer;
@property (nonatomic, strong, nullable) id localEventMonitor;
@property (nonatomic, strong, nullable) id globalEventMonitor;
@property (nonatomic) LSServiceStatus localStatus;
@property (nonatomic, copy) NSString *localMessage;

@end

@implementation LSStatusItemController

- (instancetype)init {
    self = [super init];
    if (!self) {
        return nil;
    }

    _localStatus = LSServiceStatusIdle;
    _localMessage = @"";

    _contentController = [[LSStatusPopoverViewController alloc] init];
    __weak typeof(self) weakSelf = self;
    _contentController.onOpenDashboard = ^{
        [weakSelf dismissPopover];
        if (weakSelf.onOpenDashboard) {
            weakSelf.onOpenDashboard();
        }
    };
    _contentController.onRetry = ^{
        [weakSelf dismissPopover];
        if (weakSelf.onRetry) {
            weakSelf.onRetry();
        }
    };
    _contentController.onQuit = ^{
        [weakSelf dismissPopover];
        if (weakSelf.onQuit) {
            weakSelf.onQuit();
        }
    };

    _panel = [[LSStatusPanel alloc] initWithContentRect:NSMakeRect(0, 0, _contentController.preferredContentSize.width, _contentController.preferredContentSize.height)
                                              styleMask:NSWindowStyleMaskBorderless
                                                backing:NSBackingStoreBuffered
                                                  defer:NO];
    _panel.contentViewController = _contentController;
    _panel.releasedWhenClosed = NO;
    _panel.floatingPanel = YES;
    _panel.hidesOnDeactivate = NO;
    _panel.level = NSPopUpMenuWindowLevel;
    _panel.backgroundColor = [NSColor clearColor];
    _panel.opaque = NO;
    _panel.hasShadow = YES;
    _panel.animationBehavior = NSWindowAnimationBehaviorNone;
    _panel.movable = NO;
    _panel.movableByWindowBackground = NO;
    _panel.collectionBehavior = NSWindowCollectionBehaviorTransient | NSWindowCollectionBehaviorMoveToActiveSpace;

    _statusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSSquareStatusItemLength];
    _statusItem.button.image = [self statusItemImage];
    _statusItem.button.image.template = YES;
    _statusItem.button.target = self;
    _statusItem.button.action = @selector(togglePopover:);
    _statusItem.button.toolTip = @"LlamaSitter";

    return self;
}

- (void)updateLocalStatus:(LSServiceStatus)status message:(nullable NSString *)message {
    self.localStatus = status;
    self.localMessage = message ?: @"";
    [self.contentController updateLocalStatus:status message:message];
    self.statusItem.button.toolTip = [NSString stringWithFormat:@"LlamaSitter: %@", [self statusStringForStatus:status]];
}

- (void)applyOverviewSnapshot:(nullable LSOverviewSnapshot *)snapshot {
    [self.contentController applyOverviewSnapshot:snapshot];
}

- (void)dismissPopover {
    if (self.panel.visible) {
        [self.panel orderOut:nil];
    }
    [self removeEventMonitors];
    [self stopRefreshTimer];
}

- (void)togglePopover:(__unused id)sender {
    if (self.panel.visible) {
        [self dismissPopover];
        return;
    }

    NSStatusBarButton *button = self.statusItem.button;
    if (!button) {
        return;
    }

    [self.contentController updateLocalStatus:self.localStatus message:self.localMessage];
    if (self.onRefreshRequested) {
        self.onRefreshRequested();
    }

    [NSApp activateIgnoringOtherApps:YES];
    [self positionPanelRelativeToStatusButton:button];
    [self.panel orderFront:nil];
    [self.panel makeKeyAndOrderFront:nil];
    dispatch_async(dispatch_get_main_queue(), ^{
        [self installEventMonitors];
    });
    [self startRefreshTimer];
}

- (void)refreshTimerFired:(__unused NSTimer *)timer {
    if (self.onRefreshRequested) {
        self.onRefreshRequested();
    }
}

- (void)startRefreshTimer {
    [self stopRefreshTimer];
    self.refreshTimer = [NSTimer scheduledTimerWithTimeInterval:5.0
                                                         target:self
                                                       selector:@selector(refreshTimerFired:)
                                                       userInfo:nil
                                                        repeats:YES];
}

- (void)stopRefreshTimer {
    [self.refreshTimer invalidate];
    self.refreshTimer = nil;
}

- (NSString *)statusStringForStatus:(LSServiceStatus)status {
    switch (status) {
        case LSServiceStatusStarting:
            return @"Starting";
        case LSServiceStatusReady:
            return @"Ready";
        case LSServiceStatusFailed:
            return @"Error";
        case LSServiceStatusIdle:
        default:
            return @"Stopped";
    }
}

- (NSImage *)statusItemImage {
    NSURL *resourceURL = [[NSBundle mainBundle] URLForResource:@"MenuBarIcon" withExtension:@"png"];
    NSImage *image = resourceURL ? [[NSImage alloc] initWithContentsOfURL:resourceURL] : nil;
    if (image) {
        image.template = YES;
        image.size = NSMakeSize(18, 18);
        return image;
    }

    NSImage *fallback = [[NSImage alloc] initWithSize:NSMakeSize(18, 18)];
    fallback.template = YES;
    return fallback;
}

- (void)positionPanelRelativeToStatusButton:(NSStatusBarButton *)button {
    NSRect buttonRectInWindow = [button convertRect:button.bounds toView:nil];
    NSRect buttonRectOnScreen = [button.window convertRectToScreen:buttonRectInWindow];

    NSSize panelSize = self.contentController.preferredContentSize;
    NSScreen *screen = button.window.screen ?: NSScreen.mainScreen;
    NSRect visibleFrame = screen.visibleFrame;

    CGFloat originX = NSMidX(buttonRectOnScreen) - (panelSize.width / 2.0);
    CGFloat originY = NSMinY(buttonRectOnScreen) - panelSize.height - 6.0;

    originX = MAX(NSMinX(visibleFrame) + 8.0, MIN(originX, NSMaxX(visibleFrame) - panelSize.width - 8.0));
    originY = MAX(NSMinY(visibleFrame) + 8.0, originY);

    [self.panel setFrame:NSMakeRect(originX, originY, panelSize.width, panelSize.height) display:NO];
}

- (void)installEventMonitors {
    [self removeEventMonitors];

    __weak typeof(self) weakSelf = self;
    NSEventMask mask = NSEventMaskLeftMouseDown | NSEventMaskRightMouseDown | NSEventMaskOtherMouseDown | NSEventMaskKeyDown;

    self.localEventMonitor = [NSEvent addLocalMonitorForEventsMatchingMask:mask handler:^NSEvent * _Nullable(NSEvent *event) {
        return [weakSelf handleLocalEvent:event];
    }];

    NSEventMask globalMask = NSEventMaskLeftMouseDown | NSEventMaskRightMouseDown | NSEventMaskOtherMouseDown;
    self.globalEventMonitor = [NSEvent addGlobalMonitorForEventsMatchingMask:globalMask handler:^(__unused NSEvent *event) {
        dispatch_async(dispatch_get_main_queue(), ^{
            [weakSelf dismissPopover];
        });
    }];
}

- (void)removeEventMonitors {
    if (self.localEventMonitor) {
        [NSEvent removeMonitor:self.localEventMonitor];
        self.localEventMonitor = nil;
    }

    if (self.globalEventMonitor) {
        [NSEvent removeMonitor:self.globalEventMonitor];
        self.globalEventMonitor = nil;
    }
}

- (NSEvent *)handleLocalEvent:(NSEvent *)event {
    if (!self.panel.visible) {
        return event;
    }

    if (event.type == NSEventTypeKeyDown && event.keyCode == 53) {
        [self dismissPopover];
        return nil;
    }

    if (event.type == NSEventTypeLeftMouseDown ||
        event.type == NSEventTypeRightMouseDown ||
        event.type == NSEventTypeOtherMouseDown) {
        NSPoint screenPoint = NSEvent.mouseLocation;
        if ([self pointIsInsidePanelOrStatusItem:screenPoint]) {
            return event;
        }

        [self dismissPopover];
    }

    return event;
}

- (BOOL)pointIsInsidePanelOrStatusItem:(NSPoint)screenPoint {
    if (NSPointInRect(screenPoint, self.panel.frame)) {
        return YES;
    }

    NSStatusBarButton *button = self.statusItem.button;
    if (!button.window) {
        return NO;
    }

    NSRect buttonRectInWindow = [button convertRect:button.bounds toView:nil];
    NSRect buttonRectOnScreen = [button.window convertRectToScreen:buttonRectInWindow];
    return NSPointInRect(screenPoint, buttonRectOnScreen);
}

@end
