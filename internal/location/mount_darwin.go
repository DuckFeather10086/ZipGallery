//go:build darwin

package location

import "syscall"

// Mounted SMB paths may live outside /Volumes. Keep their identity explicitly
// local until server/share identity and relocation can be resolved reliably.
func isMountedSMB(path string) bool {
	var st syscall.Statfs_t
	if syscall.Statfs(path, &st) != nil {
		return false
	}
	var name []byte
	for _, c := range st.Fstypename {
		if c == 0 {
			break
		}
		name = append(name, byte(c))
	}
	return string(name) == "smbfs"
}
