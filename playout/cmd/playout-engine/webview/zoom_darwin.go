//go:build !cli && darwin

package webview

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

void zoomFrontWindow() {
    NSWindow *win = [[NSApplication sharedApplication] mainWindow];
    if (win != nil) {
        [win zoom:nil];
    }
}
*/
import "C"

func zoomMainWindow() {
	C.zoomFrontWindow()
}
