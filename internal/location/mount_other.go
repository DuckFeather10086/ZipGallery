//go:build !darwin

package location

func isMountedSMB(path string) bool { return false }
