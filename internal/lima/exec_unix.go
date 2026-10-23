//go:build !windows

package lima

import "syscall"

// execve replaces the current process with the program at path, as exec(3)
// does.
func execve(path string, argv, env []string) error { return syscall.Exec(path, argv, env) }
