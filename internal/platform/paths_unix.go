//go:build !windows

package platform

func ServerBaseDir() string { return "/opt/mirage" }
func ClientBaseDir() string { return "/var/lib/mirage-client" }
