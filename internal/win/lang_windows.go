//go:build windows

package win

import (
	"pdf-watcher/internal/i18n"

	"golang.org/x/sys/windows"
)

var (
	kernel32                 = windows.NewLazySystemDLL("kernel32.dll")
	procGetUserDefaultUILang = kernel32.NewProc("GetUserDefaultUILanguage")
)

func SystemUILang() i18n.Lang {
	r, _, _ := procGetUserDefaultUILang.Call()
	lang := uint16(r)
	primary := lang & 0xff
	if primary == 0x04 { // LANG_CHINESE
		return i18n.ZhCN
	}
	return i18n.En
}
