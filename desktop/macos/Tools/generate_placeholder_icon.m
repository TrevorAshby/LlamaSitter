#import <Cocoa/Cocoa.h>

static void DrawRoundedCard(NSRect rect) {
    CGFloat inset = NSWidth(rect) * 0.08;
    NSRect innerRect = NSInsetRect(rect, inset, inset);
    CGFloat radius = NSWidth(rect) * 0.22;
    NSBezierPath *path = [NSBezierPath bezierPathWithRoundedRect:innerRect xRadius:radius yRadius:radius];

    NSColor *startColor = [NSColor colorWithCalibratedRed:0.72 green:0.36 blue:0.22 alpha:1.0];
    NSColor *endColor = [NSColor colorWithCalibratedRed:0.43 green:0.21 blue:0.13 alpha:1.0];
    NSGradient *gradient = [[NSGradient alloc] initWithStartingColor:startColor endingColor:endColor];
    [gradient drawInBezierPath:path angle:90.0];
}

static void DrawText(NSRect rect) {
    CGFloat pixels = NSWidth(rect);
    NSMutableParagraphStyle *paragraph = [[NSMutableParagraphStyle alloc] init];
    paragraph.alignment = NSTextAlignmentCenter;

    NSDictionary<NSAttributedStringKey, id> *headlineAttributes = @{
        NSFontAttributeName: [NSFont systemFontOfSize:pixels * 0.36 weight:NSFontWeightHeavy],
        NSForegroundColorAttributeName: [NSColor whiteColor],
        NSParagraphStyleAttributeName: paragraph,
    };
    NSDictionary<NSAttributedStringKey, id> *sublineAttributes = @{
        NSFontAttributeName: [NSFont systemFontOfSize:pixels * 0.11 weight:NSFontWeightSemibold],
        NSForegroundColorAttributeName: [NSColor colorWithCalibratedWhite:1.0 alpha:0.82],
        NSKernAttributeName: @(pixels * 0.018),
        NSParagraphStyleAttributeName: paragraph,
    };

    NSAttributedString *headline = [[NSAttributedString alloc] initWithString:@"LS" attributes:headlineAttributes];
    NSAttributedString *subline = [[NSAttributedString alloc] initWithString:@"SITTER" attributes:sublineAttributes];

    NSSize headlineSize = headline.size;
    NSSize sublineSize = subline.size;
    CGFloat totalHeight = headlineSize.height + sublineSize.height - pixels * 0.05;

    NSRect headlineRect = NSMakeRect(0.0, NSMidY(rect) - totalHeight * 0.35, NSWidth(rect), headlineSize.height);
    NSRect sublineRect = NSMakeRect(0.0, NSMinY(headlineRect) - sublineSize.height * 0.35, NSWidth(rect), sublineSize.height);

    [headline drawInRect:headlineRect];
    [subline drawInRect:sublineRect];
}

int main(int argc, const char *argv[]) {
    @autoreleasepool {
        if (argc != 2) {
            fprintf(stderr, "usage: generate_placeholder_icon <output-png>\n");
            return EXIT_FAILURE;
        }

        NSString *outputPath = [NSString stringWithUTF8String:argv[1]];
        NSURL *outputURL = [NSURL fileURLWithPath:outputPath];

        CGFloat pixels = 1024.0;
        NSSize size = NSMakeSize(pixels, pixels);
        NSImage *image = [[NSImage alloc] initWithSize:size];

        [image lockFocus];
        NSRect rect = NSMakeRect(0.0, 0.0, pixels, pixels);
        [[NSColor colorWithCalibratedRed:0.96 green:0.90 blue:0.84 alpha:1.0] setFill];
        NSRectFill(rect);
        DrawRoundedCard(rect);
        DrawText(rect);
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
