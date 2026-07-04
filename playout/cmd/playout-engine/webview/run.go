//go:build !cli

package webview

import wv "github.com/webview/webview_go"

// RunWebview opens a native WKWebView window and blocks until it is closed.
// Must be called from the main goroutine.
func RunWebview(url, title string) {
	w := wv.New(true)
	defer w.Destroy()
	w.SetTitle(title)
	w.SetSize(440, 800, wv.HintFixed)
	w.Navigate(url)
	w.Dispatch(zoomMainWindow)
	w.Run()
}
