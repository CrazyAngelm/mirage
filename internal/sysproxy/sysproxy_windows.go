//go:build windows

package sysproxy

import (
	"fmt"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

var (
	wininet                = syscall.NewLazyDLL("wininet.dll")
	procInternetSetOptionA = wininet.NewProc("InternetSetOptionA")

	shell32           = syscall.NewLazyDLL("shell32.dll")
	procIsUserAnAdmin = shell32.NewProc("IsUserAnAdmin")
)

const (
	internetOptionSettingsChanged = 39
	internetOptionRefresh         = 37
)

func Set(proxy string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open registry: %w", err)
	}
	defer k.Close()

	if err := k.SetDWordValue("ProxyEnable", 1); err != nil {
		return fmt.Errorf("set ProxyEnable: %w", err)
	}
	if err := k.SetStringValue("ProxyServer", proxy); err != nil {
		return fmt.Errorf("set ProxyServer: %w", err)
	}

	refreshWinInet()
	return nil
}

func Unset() error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open registry: %w", err)
	}
	defer k.Close()

	if err := k.SetDWordValue("ProxyEnable", 0); err != nil {
		return fmt.Errorf("set ProxyEnable: %w", err)
	}

	refreshWinInet()
	return nil
}

func refreshWinInet() {
	procInternetSetOptionA.Call(0, uintptr(internetOptionSettingsChanged), 0, 0)
	procInternetSetOptionA.Call(0, uintptr(internetOptionRefresh), 0, 0)
}

func IsAdmin() bool {
	ret, _, _ := procIsUserAnAdmin.Call()
	return ret != 0
}
