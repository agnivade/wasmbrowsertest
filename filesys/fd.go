//go:build darwin || linux

package filesys

func FdType(fd int) int {
	return fd
}
