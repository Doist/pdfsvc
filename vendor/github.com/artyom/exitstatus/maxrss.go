// +build !linux

package exitstatus

import "syscall"

func maxrss(r *syscall.Rusage) int64 { return r.Maxrss }
