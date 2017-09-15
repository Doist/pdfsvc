package exitstatus

import "syscall"

// maxrss on linux returns size in kilobytes (getrusage(2)):
//
//	ru_maxrss (since Linux 2.6.32)
//	This is the maximum resident set size used (in kilobytes).
func maxrss(r *syscall.Rusage) int64 { return r.Maxrss << 10 }
