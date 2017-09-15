// Package exitstatus provides helper functions to print status of finished
// commands started by os/exec package in a human-friendly way.
package exitstatus

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/artyom/bytesize"
)

// Reason translates error returned by os.Process.Wait() into human-readable
// string.
func Reason(err error) string {
	if err == nil {
		return "exit code 0"
	}
	exiterr, ok := err.(*exec.ExitError)
	if !ok {
		return err.Error()
	}
	status := exiterr.Sys().(syscall.WaitStatus)
	switch {
	case status.Exited():
		return fmt.Sprintf("exit code %d", status.ExitStatus())
	case status.Signaled():
		return fmt.Sprintf("exit code %d (%s)",
			128+int(status.Signal()), status.Signal())
	}
	return err.Error()
}

// Stats returns finished process' CPU / memory statistics in
// human-friendly form.
func Stats(st *os.ProcessState) string {
	if st == nil {
		return "n/a"
	}
	if r, ok := st.SysUsage().(*syscall.Rusage); ok && r != nil {
		return fmt.Sprintf("sys: %v, user: %v, maxRSS: %v",
			st.SystemTime().Round(time.Millisecond),
			st.UserTime().Round(time.Millisecond),
			bytesize.Bytes(maxrss(r)),
		)
	}
	return fmt.Sprintf("sys: %v, user: %v",
		st.SystemTime().Round(time.Millisecond),
		st.UserTime().Round(time.Millisecond))
}
