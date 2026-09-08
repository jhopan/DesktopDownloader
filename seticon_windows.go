package main

import (
	"syscall"
)

// setWindowIcon: pasang icon dari resource exe (ID 1, dibuat go-winres)
// ke title bar window. Tanpa ini, WebView2 pakai icon default (hijau).
var (
	user32      = syscall.NewLazyDLL("user32.dll")
	kernel32    = syscall.NewLazyDLL("kernel32.dll")
	procLoadImg = user32.NewProc("LoadImageW")
	procSendMsg = user32.NewProc("SendMessageW")
	procGetMod  = kernel32.NewProc("GetModuleHandleW")
)

const (
	IMAGE_ICON  = 1
	LR_DEFAULTSIZE = 0x00000040
	WM_SETICON  = 0x0080
	ICON_SMALL  = 0
	ICON_BIG    = 1
)

func setWindowIcon(hwnd uintptr) {
	hmod, _, _ := procGetMod.Call(0)
	// IDI resource pertama dari .syso (go-winres: APP icon) = ID 1
	hIcon, _, _ := procLoadImg.Call(hmod, 1, IMAGE_ICON, 32, 32, LR_DEFAULTSIZE)
	if hIcon != 0 {
		procSendMsg.Call(hwnd, WM_SETICON, ICON_BIG, hIcon)
	}
	hIconS, _, _ := procLoadImg.Call(hmod, 1, IMAGE_ICON, 16, 16, LR_DEFAULTSIZE)
	if hIconS != 0 {
		procSendMsg.Call(hwnd, WM_SETICON, ICON_SMALL, hIconS)
	}
}


