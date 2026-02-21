// nstoolbar_darwin.m — Native NSToolbar with SF Symbols for GoPad (macOS only).
// Built as part of the CGo compilation unit; linked via nstoolbar_darwin.go.

#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>

// ── Toolbar item identifiers ────────────────────────────────────────────────

static NSToolbarItemIdentifier const GoPadSidebarItemID   = @"com.gopad.sidebar";
static NSToolbarItemIdentifier const GoPadOpenItemID      = @"com.gopad.open";
static NSToolbarItemIdentifier const GoPadNewItemID       = @"com.gopad.new";
static NSToolbarItemIdentifier const GoPadFormatItemID    = @"com.gopad.format";
static NSToolbarItemIdentifier const GoPadRunItemID       = @"com.gopad.run";
static NSToolbarItemIdentifier const GoPadRerunItemID     = @"com.gopad.rerun";

// Tags for identification when updating state.
enum {
    GoPadTagSidebar = 1,
    GoPadTagOpen    = 2,
    GoPadTagNew     = 3,
    GoPadTagFormat  = 4,
    GoPadTagRun     = 5,
    GoPadTagRerun   = 6,
};

// ── WKWebView finder ────────────────────────────────────────────────────────

static WKWebView *FindWKWebView(NSView *root) {
    if ([root isKindOfClass:[WKWebView class]]) {
        return (WKWebView *)root;
    }
    for (NSView *child in root.subviews) {
        WKWebView *found = FindWKWebView(child);
        if (found) return found;
    }
    return nil;
}

// ── Toolbar delegate ────────────────────────────────────────────────────────

@interface GoPadToolbarDelegate : NSObject <NSToolbarDelegate>
@end

@implementation GoPadToolbarDelegate

- (void)toolbarAction:(NSToolbarItem *)sender {
    NSString *action = nil;
    switch (sender.tag) {
        case GoPadTagSidebar: action = @"toggleSidebar"; break;
        case GoPadTagOpen:    action = @"openFolder";     break;
        case GoPadTagNew:     action = @"newSnippet";     break;
        case GoPadTagFormat:  action = @"format";         break;
        case GoPadTagRun:     action = @"run";            break;
        case GoPadTagRerun:   action = @"rerun";          break;
        default: return;
    }

    NSWindow *window = [[NSApplication sharedApplication] mainWindow];
    if (!window) return;

    WKWebView *webView = FindWKWebView(window.contentView);
    if (!webView) return;

    NSString *js = [NSString stringWithFormat:
        @"if(window.__gopadToolbarAction){window.__gopadToolbarAction('%@')}", action];
    [webView evaluateJavaScript:js completionHandler:nil];
}

- (NSToolbarItem *)toolbar:(NSToolbar *)toolbar
     itemForItemIdentifier:(NSToolbarItemIdentifier)itemIdentifier
 willBeInsertedIntoToolbar:(BOOL)flag {

    NSToolbarItem *item = [[NSToolbarItem alloc] initWithItemIdentifier:itemIdentifier];
    item.target = self;
    item.action = @selector(toolbarAction:);

    if ([itemIdentifier isEqualToString:GoPadSidebarItemID]) {
        item.label = @"Sidebar";
        item.tag = GoPadTagSidebar;
        if (@available(macOS 11.0, *)) {
            item.image = [NSImage imageWithSystemSymbolName:@"sidebar.leading"
                                  accessibilityDescription:@"Toggle Sidebar"];
        }
    } else if ([itemIdentifier isEqualToString:GoPadOpenItemID]) {
        item.label = @"Open";
        item.tag = GoPadTagOpen;
        if (@available(macOS 11.0, *)) {
            item.image = [NSImage imageWithSystemSymbolName:@"folder"
                                  accessibilityDescription:@"Open Folder"];
        }
    } else if ([itemIdentifier isEqualToString:GoPadNewItemID]) {
        item.label = @"New";
        item.tag = GoPadTagNew;
        if (@available(macOS 11.0, *)) {
            item.image = [NSImage imageWithSystemSymbolName:@"doc.badge.plus"
                                  accessibilityDescription:@"New Snippet"];
        }
    } else if ([itemIdentifier isEqualToString:GoPadFormatItemID]) {
        item.label = @"Format";
        item.tag = GoPadTagFormat;
        if (@available(macOS 11.0, *)) {
            item.image = [NSImage imageWithSystemSymbolName:@"text.alignleft"
                                  accessibilityDescription:@"Format Code"];
        }
    } else if ([itemIdentifier isEqualToString:GoPadRunItemID]) {
        item.label = @"Run";
        item.tag = GoPadTagRun;
        if (@available(macOS 11.0, *)) {
            item.image = [NSImage imageWithSystemSymbolName:@"play.fill"
                                  accessibilityDescription:@"Run Snippet"];
        }
    } else if ([itemIdentifier isEqualToString:GoPadRerunItemID]) {
        item.label = @"Re-run";
        item.tag = GoPadTagRerun;
        if (@available(macOS 11.0, *)) {
            item.image = [NSImage imageWithSystemSymbolName:@"arrow.clockwise"
                                  accessibilityDescription:@"Re-run Last"];
        }
    }

    return item;
}

- (NSArray<NSToolbarItemIdentifier> *)toolbarAllowedItemIdentifiers:(NSToolbar *)toolbar {
    return @[
        GoPadSidebarItemID,
        GoPadOpenItemID,
        GoPadNewItemID,
        GoPadFormatItemID,
        GoPadRunItemID,
        GoPadRerunItemID,
        NSToolbarFlexibleSpaceItemIdentifier,
    ];
}

- (NSArray<NSToolbarItemIdentifier> *)toolbarDefaultItemIdentifiers:(NSToolbar *)toolbar {
    return @[
        GoPadSidebarItemID,
        GoPadOpenItemID,
        GoPadNewItemID,
        GoPadFormatItemID,
        GoPadRunItemID,
        NSToolbarFlexibleSpaceItemIdentifier,
        GoPadRerunItemID,
    ];
}

@end

// ── Stored delegate reference (prevent ARC release) ─────────────────────────

static GoPadToolbarDelegate *_sharedDelegate = nil;

// ── Public C functions called from Go via CGo ───────────────────────────────

void SetupNativeToolbar(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSArray<NSWindow *> *windows = [[NSApplication sharedApplication] windows];
        if (windows.count == 0) return;

        NSWindow *window = windows[0];

        // Remove Wails' default empty toolbar if present.
        if (window.toolbar) {
            window.toolbar = nil;
        }

        _sharedDelegate = [[GoPadToolbarDelegate alloc] init];

        NSToolbar *toolbar = [[NSToolbar alloc] initWithIdentifier:@"com.gopad.toolbar"];
        toolbar.delegate = _sharedDelegate;
        toolbar.displayMode = NSToolbarDisplayModeIconOnly;
        toolbar.allowsUserCustomization = NO;

        window.toolbar = toolbar;
        window.titleVisibility = NSWindowTitleHidden;

        if (@available(macOS 11.0, *)) {
            window.toolbarStyle = NSWindowToolbarStyleUnified;
        }
    });
}

void UpdateToolbarRunState(int isRunning) {
    dispatch_async(dispatch_get_main_queue(), ^{
        NSArray<NSWindow *> *windows = [[NSApplication sharedApplication] windows];
        if (windows.count == 0) return;

        NSWindow *window = windows[0];
        NSToolbar *toolbar = window.toolbar;
        if (!toolbar) return;

        for (NSToolbarItem *item in toolbar.items) {
            if (item.tag == GoPadTagRun) {
                if (@available(macOS 11.0, *)) {
                    if (isRunning) {
                        item.image = [NSImage imageWithSystemSymbolName:@"stop.fill"
                                              accessibilityDescription:@"Stop Run"];
                        item.label = @"Stop";
                    } else {
                        item.image = [NSImage imageWithSystemSymbolName:@"play.fill"
                                              accessibilityDescription:@"Run Snippet"];
                        item.label = @"Run";
                    }
                }
                break;
            }
        }
    });
}
