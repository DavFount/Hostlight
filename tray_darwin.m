#import <AppKit/AppKit.h>

extern void hostlightStatusMenuAction(int quit);

@interface HostlightStatusMenu : NSObject
- (void)showHostlight:(id)sender;
- (void)quitHostlight:(id)sender;
@end

@implementation HostlightStatusMenu
- (void)showHostlight:(id)sender {
    hostlightStatusMenuAction(0);
}
- (void)quitHostlight:(id)sender {
    hostlightStatusMenuAction(1);
}
@end

static NSStatusItem *statusItem;
static HostlightStatusMenu *menuTarget;

int hostlightStartStatusMenu(void) {
    __block int available = 0;
    dispatch_sync(dispatch_get_main_queue(), ^{
        statusItem = [[[NSStatusBar systemStatusBar]
                       statusItemWithLength:NSVariableStatusItemLength] retain];
        if (!statusItem) {
            return;
        }

        menuTarget = [[HostlightStatusMenu alloc] init];
        NSMenu *menu = [[NSMenu alloc] initWithTitle:@"Hostlight"];
        NSMenuItem *show = [menu addItemWithTitle:@"Show Hostlight"
                                          action:@selector(showHostlight:) keyEquivalent:@""];
        show.target = menuTarget;
        [menu addItem:[NSMenuItem separatorItem]];
        NSMenuItem *quit = [menu addItemWithTitle:@"Quit Hostlight"
                                          action:@selector(quitHostlight:) keyEquivalent:@""];
        quit.target = menuTarget;
        statusItem.menu = menu;
        [menu release];

        NSStatusBarButton *button = statusItem.button;
        button.image = [NSImage imageWithSystemSymbolName:@"waveform.path.ecg"
                                 accessibilityDescription:@"Hostlight"];
        if (!button.image) {
            button.title = @"Hostlight";
        }
        button.toolTip = @"Hostlight";
        available = 1;
    });
    return available;
}

void hostlightStopStatusMenu(void) {
    // Wails may call shutdown on the main thread.
    dispatch_async(dispatch_get_main_queue(), ^{
        [[NSStatusBar systemStatusBar] removeStatusItem:statusItem];
        [statusItem release];
        statusItem = nil;
        [menuTarget release];
        menuTarget = nil;
    });
}
