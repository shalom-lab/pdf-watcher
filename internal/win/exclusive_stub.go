//go:build !windows

package win

import (
	"os"

	"pdf-watcher/internal/i18n"
)

func TryExclusiveOpen(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	return f.Close()
}

func IsHiddenOrSystem(path string) bool { return false }

func SingleInstance(name string) (func(), bool) {
	return func() {}, true
}

func AutoStartEnabledReg(valueName string) bool { return false }

func SetAutoStartReg(valueName, exePath string, enable bool) error { return nil }

func PickFolder(title string) (string, error) { return "", nil }

func OpenNotepad(path string) error { return nil }

func OpenExplorer(dir string) error { return nil }

func OpenFile(path string) error { return nil }

func SystemUILang() i18n.Lang { return i18n.En }
