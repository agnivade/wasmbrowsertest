package filesys

import "syscall"

func FdType(fd int) syscall.Handle {
	return syscall.Handle(fd)
}
