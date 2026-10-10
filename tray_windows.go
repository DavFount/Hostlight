//go:build windows

package main

import (
	_ "embed"
	"encoding/binary"
	"fmt"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

//go:embed build/windows/icon.ico
var trayIconBytes []byte

var trayUser32 = windows.NewLazySystemDLL("user32.dll")
var trayShell32 = windows.NewLazySystemDLL("shell32.dll")
var trayNotify = trayShell32.NewProc("Shell_NotifyIconW")
var trayPost = trayUser32.NewProc("PostMessageW")
var trayDestroyIcon = trayUser32.NewProc("DestroyIcon")

const trayCallback = 0x8000 + 21
const trayClose = 0x8000 + 22

type trayPoint struct{ X, Y int32 }
type trayMessage struct {
	Window         uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Point          trayPoint
	Private        uint32
}
type trayWindowClass struct {
	Size, Style                        uint32
	Proc                               uintptr
	ClassExtra, WindowExtra            int32
	Instance, Icon, Cursor, Background uintptr
	Menu, Name                         *uint16
	SmallIcon                          uintptr
}

// Full Unicode NOTIFYICONDATA, with natural pointer alignment on Windows.
type trayIconData struct {
	Size                uint32
	Window              uintptr
	ID, Flags, Callback uint32
	Icon                uintptr
	Tip                 [128]uint16
	State, StateMask    uint32
	Info                [256]uint16
	Version             uint32
	InfoTitle           [64]uint16
	InfoFlags           uint32
	GUID                windows.GUID
	BalloonIcon         uintptr
}
type nativeTray struct {
	window uintptr
	done   chan struct{}
	once   sync.Once
}

func (t *nativeTray) Close() {
	t.once.Do(func() { trayPost.Call(t.window, trayClose, 0, 0) })
	select {
	case <-t.done:
	case <-time.After(2 * time.Second):
	}
}

// Select an existing 32px PNG frame, validating all offsets before native access.
func trayIconFrame(data []byte) ([]byte, error) {
	if len(data) < 6 || binary.LittleEndian.Uint16(data[:2]) != 0 || binary.LittleEndian.Uint16(data[2:4]) != 1 {
		return nil, fmt.Errorf("Invalid tray icon")
	}
	count := int(binary.LittleEndian.Uint16(data[4:6]))
	if count == 0 || 6+16*count > len(data) {
		return nil, fmt.Errorf("Invalid tray icon directory")
	}
	for i := 0; i < count; i++ {
		entry := data[6+16*i : 6+16*(i+1)]
		if entry[0] != 32 || entry[1] != 32 {
			continue
		}
		size := uint64(binary.LittleEndian.Uint32(entry[8:12]))
		offset := uint64(binary.LittleEndian.Uint32(entry[12:16]))
		if size == 0 || offset < uint64(6+16*count) || offset+size > uint64(len(data)) {
			return nil, fmt.Errorf("Invalid tray icon frame")
		}
		return data[int(offset):int(offset+size)], nil
	}
	return nil, fmt.Errorf("Tray icon has no 32px frame")
}
func createTrayIcon() (uintptr, error) {
	frame, err := trayIconFrame(trayIconBytes)
	if err != nil {
		return 0, err
	}
	icon, _, err := trayUser32.NewProc("CreateIconFromResourceEx").Call(uintptr(unsafe.Pointer(&frame[0])), uintptr(len(frame)), 1, 0x30000, 32, 32, 0)
	runtime.KeepAlive(frame)
	if icon == 0 {
		return 0, fmt.Errorf("Could not create tray icon: %v", err)
	}
	return icon, nil
}
func startNativeTray(show, quit func(), availability func(bool)) (trayHandle, error) {
	type result struct {
		tray *nativeTray
		err  error
	}
	ready := make(chan result, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		t := &nativeTray{done: make(chan struct{})}
		defer close(t.done)
		icon, err := createTrayIcon()
		if err != nil {
			ready <- result{err: err}
			return
		}
		defer trayDestroyIcon.Call(icon)
		instance, _, err := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetModuleHandleW").Call(0)
		if instance == 0 {
			ready <- result{err: err}
			return
		}
		className, _ := windows.UTF16PtrFromString("HostlightTrayWindow")
		taskbarName, _ := windows.UTF16PtrFromString("TaskbarCreated")
		taskbarCreated, _, _ := trayUser32.NewProc("RegisterWindowMessageW").Call(uintptr(unsafe.Pointer(taskbarName)))
		data := trayIconData{ID: 1, Flags: 1 | 2 | 4 | 0x80, Callback: trayCallback, Icon: icon}
		data.Size = uint32(unsafe.Sizeof(data))
		copy(data.Tip[:], windows.StringToUTF16("Hostlight · Monitoring continues while hidden"))
		added := false
		notify := func(action uintptr) bool {
			ok, _, _ := trayNotify.Call(action, uintptr(unsafe.Pointer(&data)))
			return ok != 0
		}
		add := func() bool {
			if !notify(0) {
				return false
			}
			data.Version = 4
			if !notify(4) {
				notify(2)
				return false
			}
			added = true
			availability(true)
			return true
		}
		callback := windows.NewCallback(func(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
			switch msg {
			case trayClose:
				trayUser32.NewProc("DestroyWindow").Call(hwnd)
				return 0
			case 2:
				trayUser32.NewProc("PostQuitMessage").Call(0)
				return 0
			case 0x11: // Allow Windows session shutdown; don't trap sign-out behind close-to-tray.
				return 1
			case 0x16:
				if wparam != 0 {
					go quit()
				}
				return 0
			case 0x113: // Check tray reachability and retry after Explorer restarts.
				if added && !notify(1) {
					added = false
					availability(false)
				}
				if !added {
					add()
				}
				return 0
			case trayCallback:
				switch uint16(lparam) {
				case 0x400, 0x401, 0x203:
					go show()
				case 0x7b: // WM_CONTEXTMENU (mouse or keyboard).
					menu, _, _ := trayUser32.NewProc("CreatePopupMenu").Call()
					if menu == 0 {
						return 0
					}
					defer trayUser32.NewProc("DestroyMenu").Call(menu)
					showText, _ := windows.UTF16PtrFromString("Show Hostlight")
					quitText, _ := windows.UTF16PtrFromString("Quit Hostlight (stop monitoring)")
					appendMenu := trayUser32.NewProc("AppendMenuW")
					appendMenu.Call(menu, 0, 1, uintptr(unsafe.Pointer(showText)))
					appendMenu.Call(menu, 0x800, 0, 0)
					appendMenu.Call(menu, 0, 2, uintptr(unsafe.Pointer(quitText)))
					var point trayPoint
					trayUser32.NewProc("GetCursorPos").Call(uintptr(unsafe.Pointer(&point)))
					trayUser32.NewProc("SetForegroundWindow").Call(hwnd)
					command, _, _ := trayUser32.NewProc("TrackPopupMenu").Call(menu, 0x100|0x2, uintptr(point.X), uintptr(point.Y), 0, hwnd, 0)
					trayPost.Call(hwnd, 0, 0, 0)
					notify(3)
					if command == 1 {
						go show()
					}
					if command == 2 {
						go quit()
					}
				}
				return 0
			}
			if taskbarCreated != 0 && uintptr(msg) == taskbarCreated {
				added = false
				availability(false)
				add()
				return 0
			}
			r, _, _ := trayUser32.NewProc("DefWindowProcW").Call(hwnd, uintptr(msg), wparam, lparam)
			return r
		})
		wc := trayWindowClass{Proc: callback, Instance: uintptr(instance), Name: className}
		wc.Size = uint32(unsafe.Sizeof(wc))
		atom, _, err := trayUser32.NewProc("RegisterClassExW").Call(uintptr(unsafe.Pointer(&wc)))
		if atom == 0 {
			ready <- result{err: fmt.Errorf("Could not register tray window: %v", err)}
			return
		}
		defer trayUser32.NewProc("UnregisterClassW").Call(uintptr(unsafe.Pointer(className)), uintptr(instance))
		hwnd, _, err := trayUser32.NewProc("CreateWindowExW").Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(className)), 0, 0, 0, 0, 0, 0, 0, uintptr(instance), 0)
		if hwnd == 0 {
			ready <- result{err: fmt.Errorf("Could not create tray window: %v", err)}
			return
		}
		t.window = hwnd
		data.Window = hwnd
		defer func() { notify(2); availability(false); trayUser32.NewProc("DestroyWindow").Call(hwnd) }()
		if !add() {
			ready <- result{err: fmt.Errorf("Windows notification area is unavailable")}
			return
		}
		timer, _, _ := trayUser32.NewProc("SetTimer").Call(hwnd, 1, 5000, 0)
		if timer == 0 {
			ready <- result{err: fmt.Errorf("Could not monitor tray availability")}
			return
		}
		defer trayUser32.NewProc("KillTimer").Call(hwnd, 1)
		ready <- result{tray: t}
		var message trayMessage
		for {
			r, _, _ := trayUser32.NewProc("GetMessageW").Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
			if int32(r) <= 0 {
				break
			}
			trayUser32.NewProc("TranslateMessage").Call(uintptr(unsafe.Pointer(&message)))
			trayUser32.NewProc("DispatchMessageW").Call(uintptr(unsafe.Pointer(&message)))
		}
	}()
	r := <-ready
	return r.tray, r.err
}
