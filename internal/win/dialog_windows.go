//go:build windows

package win

import (
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	ole32                    = windows.NewLazySystemDLL("ole32.dll")
	shell32                  = windows.NewLazySystemDLL("shell32.dll")
	procCoInitializeEx       = ole32.NewProc("CoInitializeEx")
	procCoUninitialize       = ole32.NewProc("CoUninitialize")
	procCoTaskMemFree        = ole32.NewProc("CoTaskMemFree")
	procSHBrowseForFolderW   = shell32.NewProc("SHBrowseForFolderW")
	procSHGetPathFromIDListW = shell32.NewProc("SHGetPathFromIDListW")
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

const (
	bifReturnOnlyFSDirs = 0x00000001
	bifNewDialogStyle   = 0x00000040
	coinitApartment     = 0x2
)

func PickFolder(title string) (string, error) {
	procCoInitializeEx.Call(0, coinitApartment)
	defer procCoUninitialize.Call()

	buf := make([]uint16, windows.MAX_PATH)
	titlePtr, _ := windows.UTF16PtrFromString(title)
	bi := browseInfo{
		pszDisplayName: &buf[0],
		lpszTitle:      titlePtr,
		ulFlags:        bifReturnOnlyFSDirs | bifNewDialogStyle,
	}
	r, _, _ := procSHBrowseForFolderW.Call(uintptr(unsafe.Pointer(&bi)))
	if r == 0 {
		return "", nil
	}
	defer procCoTaskMemFree.Call(r)
	out := make([]uint16, windows.MAX_PATH)
	ok, _, _ := procSHGetPathFromIDListW.Call(r, uintptr(unsafe.Pointer(&out[0])))
	if ok == 0 {
		return "", syscall.EINVAL
	}
	return windows.UTF16ToString(out), nil
}

func OpenNotepad(path string) error {
	return exec.Command("notepad.exe", path).Start()
}

func OpenExplorer(dir string) error {
	return exec.Command("explorer.exe", dir).Start()
}

// OpenFile opens path with the default associated application.
func OpenFile(path string) error {
	// Avoid cmd.exe metacharacters; FileProtocolHandler takes the path as one arg.
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", path).Start()
}
