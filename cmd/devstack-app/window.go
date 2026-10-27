package main

import (
	"log"
	"runtime"
	"sync"
	"unsafe"

	webview2 "github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"github.com/harsha2805/devstack/internal/winproc"
)

// window owns the dashboard window. Closing it only hides devstack to the
// tray; Show opens a new one or brings the existing one forward.
type window struct {
	url     string
	dataDir string

	mu   sync.Mutex
	open bool
	hwnd uintptr
}

// Default and minimum window size in 96-DPI pixels.
const (
	winW, winH       = 1000, 700
	minWinW, minWinH = 720, 540
)

var (
	user32               = windows.NewLazySystemDLL("user32.dll")
	procShowWindow       = user32.NewProc("ShowWindow")
	procSetForeground    = user32.NewProc("SetForegroundWindow")
	procIsIconic         = user32.NewProc("IsIconic")
	procGetDpiForSystem  = user32.NewProc("GetDpiForSystem")
)

func (w *window) Show() {
	w.mu.Lock()
	if w.open {
		h := w.hwnd
		w.mu.Unlock()
		if h != 0 {
			if r, _, _ := procIsIconic.Call(h); r != 0 {
				procShowWindow.Call(h, 9) // SW_RESTORE
			}
			procSetForeground.Call(h)
		}
		return
	}
	w.open = true
	w.mu.Unlock()

	go func() {
		// A window belongs to the thread that made it; keep this goroutine on it.
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer func() {
			w.mu.Lock()
			w.open, w.hwnd = false, 0
			w.mu.Unlock()
		}()

		scale := dpiScale()
		wv := webview2.NewWithOptions(webview2.WebViewOptions{
			DataPath:  w.dataDir,
			AutoFocus: true,
			WindowOptions: webview2.WindowOptions{
				Title:  "devstack",
				Width:  uint(winW * scale),
				Height: uint(winH * scale),
				IconId: 1,
				Center: true,
			},
		})
		if wv == nil {
			// No WebView2 runtime: the same dashboard works in the browser.
			log.Print("WebView2 unavailable; opening the dashboard in the browser")
			winproc.ShellOpen(w.url)
			return
		}
		w.mu.Lock()
		w.hwnd = uintptr(wv.Window())
		w.mu.Unlock()
		themeTitleBar(w.hwnd)
		wv.SetSize(int(minWinW*scale), int(minWinH*scale), webview2.HintMin)
		// Opened from the tray or a second launch: come to the front. Windows
		// allows it when the user just acted (see winproc.AllowForeground).
		procSetForeground.Call(w.hwnd)
		wv.Navigate(w.url)
		wv.Run() // returns when the window is closed
	}()
}

var (
	dwmapi                    = windows.NewLazySystemDLL("dwmapi.dll")
	procDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")
)

// themeTitleBar paints the caption in the dashboard's window colour for the
// current Windows app theme, instead of the system accent. Windows 10 ignores
// the colour attributes and keeps its default caption.
func themeTitleBar(hwnd uintptr) {
	const (
		useImmersiveDarkMode = 20
		captionColor         = 35
		textColor            = 36
	)
	light := appsUseLightTheme()
	dark := uint32(0)
	caption, text := rgb(0x1B, 0x1B, 0x19), rgb(0xF0, 0xED, 0xE7)
	if light {
		caption, text = rgb(0xF3, 0xF0, 0xEA), rgb(0x29, 0x25, 0x1F)
	} else {
		dark = 1
	}
	set := func(attr uint32, v *uint32) {
		procDwmSetWindowAttribute.Call(hwnd, uintptr(attr), uintptr(unsafe.Pointer(v)), 4)
	}
	set(useImmersiveDarkMode, &dark)
	set(captionColor, &caption)
	set(textColor, &text)
}

// rgb builds a Win32 COLORREF (0x00BBGGRR).
func rgb(r, g, b uint8) uint32 { return uint32(r) | uint32(g)<<8 | uint32(b)<<16 }

func appsUseLightTheme() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return true
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("AppsUseLightTheme")
	return err != nil || v != 0
}

func dpiScale() float64 {
	if procGetDpiForSystem.Find() != nil {
		return 1
	}
	dpi, _, _ := procGetDpiForSystem.Call()
	if dpi == 0 {
		return 1
	}
	return float64(dpi) / 96
}
