// nstoolbar_darwin.m — Native NSToolbar with SF Symbols for GoPoke (macOS only).
// Built as part of the CGo compilation unit; linked via nstoolbar_darwin.go.

#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>

// ── Toolbar item identifiers ────────────────────────────────────────────────

static NSToolbarItemIdentifier const GoPadSidebarItemID   = @"com.gopoke.sidebar";
static NSToolbarItemIdentifier const GoPadOpenItemID      = @"com.gopoke.open";
static NSToolbarItemIdentifier const GoPadOpenFileItemID  = @"com.gopoke.openFile";
static NSToolbarItemIdentifier const GoPadNewItemID       = @"com.gopoke.new";
static NSToolbarItemIdentifier const GoPadFormatItemID    = @"com.gopoke.format";
static NSToolbarItemIdentifier const GoPadRunItemID       = @"com.gopoke.run";
static NSToolbarItemIdentifier const GoPadRerunItemID     = @"com.gopoke.rerun";
static NSToolbarItemIdentifier const GoPadShareItemID     = @"com.gopoke.share";
static NSToolbarItemIdentifier const GoPadImportItemID    = @"com.gopoke.import";
static NSToolbarItemIdentifier const GoPadSettingsItemID  = @"com.gopoke.settings";

// Tags for identification when updating state.
enum {
    GoPadTagSidebar  = 1,
    GoPadTagOpen     = 2,
    GoPadTagOpenFile = 8,
    GoPadTagNew      = 3,
    GoPadTagFormat   = 4,
    GoPadTagRun      = 5,
    GoPadTagRerun    = 6,
    GoPadTagSettings = 7,
    GoPadTagShare    = 9,
    GoPadTagImport   = 10,
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
        case GoPadTagOpen:     action = @"openFolder";     break;
        case GoPadTagOpenFile: action = @"openFile";      break;
        case GoPadTagNew:      action = @"newSnippet";    break;
        case GoPadTagFormat:  action = @"format";         break;
        case GoPadTagRun:     action = @"run";            break;
        case GoPadTagRerun:    action = @"rerun";          break;
        case GoPadTagShare:    action = @"share";           break;
        case GoPadTagImport:   action = @"import";          break;
        case GoPadTagSettings: action = @"settings";       break;
        default: return;
    }

    NSWindow *window = [[NSApplication sharedApplication] mainWindow];
    if (!window) return;

    WKWebView *webView = FindWKWebView(window.contentView);
    if (!webView) return;

    NSString *js = [NSString stringWithFormat:
        @"if(window.__gopokeToolbarAction){window.__gopokeToolbarAction('%@')}", action];
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
        item.toolTip = @"Toggle Sidebar (⌘B)";
        item.tag = GoPadTagSidebar;
        if (@available(macOS 11.0, *)) {
            item.image = [NSImage imageWithSystemSymbolName:@"sidebar.leading"
                                  accessibilityDescription:@"Toggle Sidebar"];
        }
    } else if ([itemIdentifier isEqualToString:GoPadOpenItemID]) {
        item.label = @"Open";
        item.toolTip = @"Open Go Project Folder";
        item.tag = GoPadTagOpen;
        if (@available(macOS 11.0, *)) {
            item.image = [NSImage imageWithSystemSymbolName:@"folder"
                                  accessibilityDescription:@"Open Folder"];
        }
    } else if ([itemIdentifier isEqualToString:GoPadOpenFileItemID]) {
        item.label = @"Open File";
        item.toolTip = @"Open a Single .go File";
        item.tag = GoPadTagOpenFile;
        if (@available(macOS 11.0, *)) {
            item.image = [NSImage imageWithSystemSymbolName:@"doc"
                                  accessibilityDescription:@"Open Go File"];
        }
    } else if ([itemIdentifier isEqualToString:GoPadNewItemID]) {
        item.label = @"New";
        item.toolTip = @"New Snippet";
        item.tag = GoPadTagNew;
        if (@available(macOS 11.0, *)) {
            item.image = [NSImage imageWithSystemSymbolName:@"doc.badge.plus"
                                  accessibilityDescription:@"New Snippet"];
        }
    } else if ([itemIdentifier isEqualToString:GoPadFormatItemID]) {
        item.label = @"Format";
        item.toolTip = @"Format Code (goimports + gofmt)";
        item.tag = GoPadTagFormat;
        if (@available(macOS 11.0, *)) {
            item.image = [NSImage imageWithSystemSymbolName:@"text.alignleft"
                                  accessibilityDescription:@"Format Code"];
        }
    } else if ([itemIdentifier isEqualToString:GoPadRunItemID]) {
        item.label = @"Run";
        item.toolTip = @"Run Snippet (⌘↩)";
        item.tag = GoPadTagRun;
        if (@available(macOS 11.0, *)) {
            item.image = [NSImage imageWithSystemSymbolName:@"play.fill"
                                  accessibilityDescription:@"Run Snippet"];
        }
    } else if ([itemIdentifier isEqualToString:GoPadRerunItemID]) {
        item.label = @"Re-run";
        item.toolTip = @"Re-run Last Snippet";
        item.tag = GoPadTagRerun;
        if (@available(macOS 11.0, *)) {
            item.image = [NSImage imageWithSystemSymbolName:@"arrow.clockwise"
                                  accessibilityDescription:@"Re-run Last"];
        }
    } else if ([itemIdentifier isEqualToString:GoPadShareItemID]) {
        item.label = @"Share";
        item.toolTip = @"Share to Go Playground";
        item.tag = GoPadTagShare;
        if (@available(macOS 11.0, *)) {
            item.image = [NSImage imageWithSystemSymbolName:@"square.and.arrow.up"
                                  accessibilityDescription:@"Share to Playground"];
        }
    } else if ([itemIdentifier isEqualToString:GoPadImportItemID]) {
        item.label = @"Import";
        item.toolTip = @"Import from Go Playground";
        item.tag = GoPadTagImport;
        if (@available(macOS 11.0, *)) {
            item.image = [NSImage imageWithSystemSymbolName:@"square.and.arrow.down"
                                  accessibilityDescription:@"Import from Playground"];
        }
    } else if ([itemIdentifier isEqualToString:GoPadSettingsItemID]) {
        item.label = @"Settings";
        item.toolTip = @"Settings (⌘,)";
        item.tag = GoPadTagSettings;
        if (@available(macOS 11.0, *)) {
            item.image = [NSImage imageWithSystemSymbolName:@"gearshape"
                                  accessibilityDescription:@"Settings"];
        }
    }

    return item;
}

- (NSArray<NSToolbarItemIdentifier> *)toolbarAllowedItemIdentifiers:(NSToolbar *)toolbar {
    return @[
        GoPadSidebarItemID,
        GoPadOpenItemID,
        GoPadOpenFileItemID,
        GoPadNewItemID,
        GoPadFormatItemID,
        GoPadRunItemID,
        GoPadShareItemID,
        GoPadImportItemID,
        GoPadRerunItemID,
        GoPadSettingsItemID,
        NSToolbarFlexibleSpaceItemIdentifier,
    ];
}

- (NSArray<NSToolbarItemIdentifier> *)toolbarDefaultItemIdentifiers:(NSToolbar *)toolbar {
    return @[
        GoPadSidebarItemID,
        GoPadOpenItemID,
        GoPadOpenFileItemID,
        GoPadNewItemID,
        GoPadFormatItemID,
        GoPadRunItemID,
        GoPadShareItemID,
        GoPadImportItemID,
        NSToolbarFlexibleSpaceItemIdentifier,
        GoPadRerunItemID,
        GoPadSettingsItemID,
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

        NSToolbar *toolbar = [[NSToolbar alloc] initWithIdentifier:@"com.gopoke.toolbar"];
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
