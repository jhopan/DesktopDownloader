package main

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	shell32       = syscall.NewLazyDLL("shell32.dll")
	ole32         = syscall.NewLazyDLL("ole32.dll")
	comdlg32      = syscall.NewLazyDLL("comdlg32.dll")
	procSHBrowse  = shell32.NewProc("SHBrowseForFolderW")
	procSHGetPath = shell32.NewProc("SHGetPathFromIDListW")
	procCoTaskFree = ole32.NewProc("CoTaskMemFree")
	procCoInitialize = ole32.NewProc("CoInitializeEx")
	procGetOpen   = comdlg32.NewProc("GetOpenFileNameW")
)

type browseInfo struct {
	hwndOwner      uintptr
	pidlRoot       uintptr
	pszDisplayName *uint16
	lpszTitle      *uint16
	ulFlags        uint32
	lpfn           uintptr
	lParam         uintptr
	iImage         int32
}

// pickFolder menampilkan dialog pilih folder Windows native. "" = batal.
func pickFolder(title string) string {
	procCoInitialize.Call(0)

	ttl, _ := syscall.UTF16PtrFromString(title)
	display := make([]uint16, 260)
	bi := browseInfo{
		pszDisplayName: &display[0],
		lpszTitle:      ttl,
		ulFlags:        0x00000001, // BIF_RETURNONLYFSDIRS
	}
	ret, _, _ := procSHBrowse.Call(uintptr(unsafe.Pointer(&bi)))
	if ret == 0 {
		return ""
	}
	path := make([]uint16, 260)
	procSHGetPath.Call(ret, uintptr(unsafe.Pointer(&path[0])))
	procCoTaskFree.Call(ret)
	return syscall.UTF16ToString(path)
}

type openFileName struct {
	lStructSize     uintptr
	hwndOwner       uintptr
	hInstance       uintptr
	lpstrFilter     *uint16
	lpstrCustom     *uint16
	nMaxCustom      uint32
	nFilterIndex    uint32
	lpstrFile       *uint16
	nMaxFile        uint32
	lpstrFileTitle  *uint16
	nMaxFileTitle   uint32
	lpstrInitialDir *uint16
	lpstrTitle      *uint16
	flags           uint32
	fileOffset      uint16
	fileExtension   uint16
	defExt          *uint16
	custData        uintptr
	fnHook          uintptr
	templateName    *uint16
}

// pickFile menampilkan dialog pilih file (filter .txt). "" = batal.
func pickFile(title string) string {
	ttl, _ := syscall.UTF16PtrFromString(title)
	filter, _ := syscall.UTF16PtrFromString("Cookies txt\x00*.txt\x00Semua file\x00*.*\x00\x00")
	buf := make([]uint16, 32768)
	ofn := openFileName{
		lStructSize: unsafe.Sizeof(openFileName{}),
		lpstrFilter: filter,
		lpstrFile:   &buf[0],
		nMaxFile:    uint32(len(buf)),
		lpstrTitle:  ttl,
		flags:       0x00001000 | 0x00000004, // OFN_FILEMUSTEXIST | OFN_NOCHANGEDIR
	}
	ret, _, _ := procGetOpen.Call(uintptr(unsafe.Pointer(&ofn)))
	if ret == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}

// cookiesArgs: "--cookies <path>" kalau file cookies ada.
func cookiesArgs() []string {
	p := cookiePath()
	if _, err := os.Stat(p); err == nil {
		return []string{"--cookies", p}
	}
	return nil
}
