//go:build !windows

package sysproxy

func Set(proxy string) error { return nil }
func Unset() error           { return nil }
func IsAdmin() bool          { return false }
