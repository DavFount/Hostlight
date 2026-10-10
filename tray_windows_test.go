//go:build windows

package main

import (
	"encoding/binary"
	"os"
	"testing"
	"time"
	"unsafe"
)

func TestTrayNativeStructuresAndIcon(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) == 8 {
		if unsafe.Sizeof(trayIconData{}) != 976 || unsafe.Sizeof(trayWindowClass{}) != 80 || unsafe.Sizeof(trayMessage{}) != 48 {
			t.Fatal("Windows native layout mismatch")
		}
	}
	if _, err := trayIconFrame(trayIconBytes); err != nil {
		t.Fatal(err)
	}
	icon, err := createTrayIcon()
	if err != nil {
		t.Fatal(err)
	}
	trayDestroyIcon.Call(icon)
	for _, data := range [][]byte{nil, {0, 0, 1, 0, 1, 0}, append([]byte(nil), trayIconBytes[:30]...)} {
		if _, err := trayIconFrame(data); err == nil {
			t.Fatal("invalid ICO accepted")
		}
	}
	corrupt := append([]byte(nil), trayIconBytes...)
	for i := 0; i < int(binary.LittleEndian.Uint16(corrupt[4:6])); i++ {
		entry := corrupt[6+16*i : 6+16*(i+1)]
		if entry[0] == 32 {
			binary.LittleEndian.PutUint32(entry[12:16], 0xffffffff)
		}
	}
	if _, err := trayIconFrame(corrupt); err == nil {
		t.Fatal("out-of-bounds ICO accepted")
	}
}

// Explicit opt-in: briefly adds an icon to the interactive Windows desktop.
func TestNativeTrayLifecycle(t *testing.T) {
	if os.Getenv("HOSTLIGHT_TRAY_SMOKE") != "1" {
		t.Skip("interactive tray smoke is opt-in")
	}
	state := make(chan bool, 10)
	shown := make(chan struct{}, 2)
	tray, err := startNativeTray(func() { shown <- struct{}{} }, func() {}, func(available bool) { state <- available })
	if err != nil {
		t.Fatal(err)
	}
	defer tray.Close()
	select {
	case ok := <-state:
		if !ok {
			t.Fatal("tray not available")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no ready state")
	}
	native := tray.(*nativeTray)
	trayPost.Call(native.window, trayCallback, 0, 1<<16|0x400)
	select {
	case <-shown:
	case <-time.After(5 * time.Second):
		t.Fatal("tray activation did not request show")
	}
	// Remove only this test's icon, then run the normal health-check message.
	// This exercises recovery without restarting or changing the user's Explorer.
	data := trayIconData{Window: native.window, ID: 1}
	data.Size = uint32(unsafe.Sizeof(data))
	trayNotify.Call(2, uintptr(unsafe.Pointer(&data)))
	trayPost.Call(native.window, 0x113, 1, 0)
	for _, expected := range []bool{false, true} {
		select {
		case actual := <-state:
			if actual != expected {
				t.Fatalf("recovery state %v; want %v", actual, expected)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("tray recovery timed out")
		}
	}
	tray.Close()
	tray.Close()
	select {
	case <-native.done:
	case <-time.After(5 * time.Second):
		t.Fatal("tray did not stop")
	}
	select {
	case ok := <-state:
		if ok {
			t.Fatal("tray not removed")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no removed state")
	}
}
