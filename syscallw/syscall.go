package syscallw

import "syscall"

const (
	LOCK_EX = 0x2
	LOCK_NB = 0x4
	LOCK_SH = 0x1
	LOCK_UN = 0x8
)

func Flock(fd int, how int) (err error) {
	return syscall.Flock(fd, how)
}

func Dup3(oldfd int, newfd int, flags int) (err error) {
	return syscall.Dup3(oldfd, newfd, flags)
}
