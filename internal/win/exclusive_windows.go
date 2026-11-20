//go:build windows

package win

import (
	"golang.org/x/sys/windows"
)

func TryExclusiveOpen(path string) error {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	h, err := windows.CreateFile(
		p,
		windows.GENERIC_READ,
		0,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return err
	}
	return windows.CloseHandle(h)
}

func IsHiddenOrSystem(path string) bool {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	attrs, err := windows.GetFileAttributes(p)
	if err != nil {
		return false
	}
	return attrs&windows.FILE_ATTRIBUTE_HIDDEN != 0 || attrs&windows.FILE_ATTRIBUTE_SYSTEM != 0
}

func SingleInstance(name string) (release func(), ok bool) {
	n, err := windows.UTF16PtrFromString(`Local\` + name)
	if err != nil {
		return func() {}, false
	}
	h, err := windows.CreateMutex(nil, false, n)
	if err == windows.ERROR_ALREADY_EXISTS {
		if h != 0 {
			_ = windows.CloseHandle(h)
		}
		return func() {}, false
	}
	if err != nil {
		return func() {}, false
	}
	return func() { _ = windows.CloseHandle(h) }, true
}
