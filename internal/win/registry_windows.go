//go:build windows

package win

import (
	"golang.org/x/sys/windows/registry"
)

func AutoStartEnabledReg(valueName string) bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(valueName)
	return err == nil
}

func SetAutoStartReg(valueName, exePath string, enable bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if enable {
		return k.SetStringValue(valueName, exePath)
	}
	return k.DeleteValue(valueName)
}
