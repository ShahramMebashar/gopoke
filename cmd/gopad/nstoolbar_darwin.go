//go:build wails && darwin

package main

/*
#cgo LDFLAGS: -framework AppKit -framework WebKit
extern void SetupNativeToolbar(void);
extern void UpdateToolbarRunState(int);
*/
import "C"

func setupNativeToolbar() {
	C.SetupNativeToolbar()
}

func updateToolbarRunState(isRunning bool) {
	v := C.int(0)
	if isRunning {
		v = 1
	}
	C.UpdateToolbarRunState(v)
}
