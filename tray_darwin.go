package main

/*
#cgo LDFLAGS: -framework AppKit -framework Foundation
int hostlightStartStatusMenu(void);
void hostlightStopStatusMenu(void);
*/
import "C"

import (
	"fmt"
	"sync"
)

type macTray struct {
	once sync.Once
}

var macTrayCallbacks struct {
	sync.Mutex
	show func()
	quit func()
}

func startNativeTray(show, quit func(), availability func(bool)) (trayHandle, error) {
	macTrayCallbacks.Lock()
	macTrayCallbacks.show, macTrayCallbacks.quit = show, quit
	macTrayCallbacks.Unlock()

	if C.hostlightStartStatusMenu() == 0 {
		clearMacTrayCallbacks()
		return nil, fmt.Errorf("macOS could not create the Hostlight menu bar item")
	}
	availability(true)
	return &macTray{}, nil
}

func clearMacTrayCallbacks() {
	macTrayCallbacks.Lock()
	macTrayCallbacks.show, macTrayCallbacks.quit = nil, nil
	macTrayCallbacks.Unlock()
}

func (tray *macTray) Close() {
	tray.once.Do(func() {
		clearMacTrayCallbacks()
		C.hostlightStopStatusMenu()
	})
}

//export hostlightStatusMenuAction
func hostlightStatusMenuAction(quit C.int) {
	macTrayCallbacks.Lock()
	action := macTrayCallbacks.show
	if quit != 0 {
		action = macTrayCallbacks.quit
	}
	macTrayCallbacks.Unlock()
	if action != nil {
		go action()
	}
}
