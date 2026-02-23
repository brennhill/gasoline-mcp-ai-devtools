//go:build linux

package main

import "syscall"

func dup2Compat(oldfd, newfd int) error {
	if oldfd == newfd {
		return nil
	}
	return syscall.Dup3(oldfd, newfd, 0)
}
