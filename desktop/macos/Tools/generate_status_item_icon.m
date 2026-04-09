#import <Cocoa/Cocoa.h>

int main(int argc, const char *argv[]) {
    @autoreleasepool {
        if (argc != 2) {
            fprintf(stderr, "usage: generate_status_item_icon <output-png>\n");
            return EXIT_FAILURE;
        }

        NSString *outputPath = [NSString stringWithUTF8String:argv[1]];
        NSURL *outputURL = [NSURL fileURLWithPath:outputPath];

        CGFloat pixels = 64.0;
        NSImage *image = [[NSImage alloc] initWithSize:NSMakeSize(pixels, pixels)];

        [image lockFocus];

        [[NSColor clearColor] setFill];
        NSRectFill(NSMakeRect(0, 0, pixels, pixels));

        NSColor *ink = [NSColor blackColor];
        [ink setFill];

        CGFloat lineWidth = 4.0;
        NSBezierPath *frame = [NSBezierPath bezierPathWithRoundedRect:NSMakeRect(8, 10, 48, 40) xRadius:10 yRadius:10];
        frame.lineWidth = lineWidth;
        [frame stroke];

        NSArray<NSValue *> *bars = @[
            [NSValue valueWithRect:NSMakeRect(16, 18, 8, 18)],
            [NSValue valueWithRect:NSMakeRect(28, 18, 8, 12)],
            [NSValue valueWithRect:NSMakeRect(40, 18, 8, 24)],
        ];

        for (NSValue *value in bars) {
            NSRect rect = value.rectValue;
            NSBezierPath *bar = [NSBezierPath bezierPathWithRoundedRect:rect xRadius:3 yRadius:3];
            [bar fill];
        }

        [image unlockFocus];

        NSData *tiffData = image.TIFFRepresentation;
        NSBitmapImageRep *bitmap = [[NSBitmapImageRep alloc] initWithData:tiffData];
        NSData *pngData = [bitmap representationUsingType:NSBitmapImageFileTypePNG properties:@{}];

        if (![pngData writeToURL:outputURL atomically:YES]) {
            fprintf(stderr, "failed to write %s\n", argv[1]);
            return EXIT_FAILURE;
        }
    }

    return EXIT_SUCCESS;
}
