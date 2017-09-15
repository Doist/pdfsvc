// Command pdfsvc is a small wrapper around wkhtmltopdf command to expose it as
// a http service
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/net/html/charset"

	"github.com/artyom/autoflags"
	"github.com/artyom/buffering"
)

func main() {
	defaultAddr := os.Getenv("ADDR")
	if defaultAddr == "" {
		defaultAddr = "localhost:8080"
	}
	args := &struct {
		Addr    string        `flag:"addr,address to listen"`
		Timeout time.Duration `flag:"d,max time to allow wkhtmltopdf command to run"`
		Procs   int           `flag:"n,max number of concurrent processes to allow"`
		Token   string        `flag:"token,if set, check Authorization Bearer token"`
		Quiet   bool          `flag:"q,be quiet, log less"`
	}{
		Addr:    defaultAddr,
		Timeout: 5 * time.Second,
		Procs:   3,
		Token:   os.Getenv("TOKEN"),
	}
	autoflags.Parse(args)
	if args.Procs <= 0 {
		args.Procs = 1
	}
	h := &handler{gate: make(chan struct{}, args.Procs),
		d: args.Timeout, token: args.Token, noisy: !args.Quiet}
	srv := &http.Server{
		Addr:              args.Addr,
		Handler:           buffering.Handler(h, buffering.WithMaxSize(1<<20)),
		ReadHeaderTimeout: time.Second,
		ReadTimeout:       time.Minute,
		WriteTimeout:      time.Minute,
	}
	log.Fatal(srv.ListenAndServe())
}

func init() { log.SetFlags(0); log.SetPrefix(filepath.Base(os.Args[0]) + ": ") }

type handler struct {
	gate  chan struct{}
	d     time.Duration
	token string
	noisy bool
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Accept", "POST")
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if h.token != "" {
		hdr := r.Header.Get("Authorization")
		if val := strings.TrimPrefix(hdr, "Bearer "); val != hdr && val == h.token {
			goto authorized
		}
		w.Header().Set("WWW-Authenticate", "Bearer")
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
authorized:
	ct := r.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	utf8Body, err := charset.NewReader(r.Body, ct)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusUnsupportedMediaType), http.StatusUnsupportedMediaType)
		return
	}
	rd, err := h.convert(r.Context(), utf8Body)
	if err != nil {
		code := http.StatusInternalServerError
		if err == context.DeadlineExceeded {
			code = http.StatusGatewayTimeout
		}
		http.Error(w, http.StatusText(code), code)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	http.ServeContent(w, r, "", time.Now(), rd)
}

func (h *handler) convert(ctx context.Context, r io.Reader) (io.ReadSeeker, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case h.gate <- struct{}{}:
		defer func() { <-h.gate }()
	}
	if h.d > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, h.d)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, "wkhtmltopdf",
		"--disable-javascript", "--disable-local-file-access",
		"--encoding", "utf8", "-q", "-", "-")
	cmd.Stdin = r
	// FIXME: we're suggesting that returned bodies are quite small, may not
	// always be the case, but ok for controlled inputs
	out, err := cmd.Output()
	if h.noisy {
		log.Print(exitReason(err), " / ", processStats(cmd.ProcessState))
	}
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(out), nil
}

// exitReason translates error returned by os.Process.Wait() into human-readable
// string.
func exitReason(err error) string {
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

// processStats returns finished process' CPU / memory statistics in
// human-readable form.
func processStats(st *os.ProcessState) string {
	if st == nil {
		return "n/a"
	}
	if r, ok := st.SysUsage().(*syscall.Rusage); ok && r != nil {
		return fmt.Sprintf("sys: %v, user: %v, maxRSS: %v",
			st.SystemTime().Round(time.Millisecond),
			st.UserTime().Round(time.Millisecond),
			ByteSize(r.Maxrss<<10),
		)
	}
	return fmt.Sprintf("sys: %v, user: %v",
		st.SystemTime().Round(time.Millisecond),
		st.UserTime().Round(time.Millisecond))
}

// ByteSize implements Stringer interface for printing size in human-readable
// form
type ByteSize float64

func (b ByteSize) String() string {
	switch {
	case b >= YB:
		return fmt.Sprintf("%.2fYB", b/YB)
	case b >= ZB:
		return fmt.Sprintf("%.2fZB", b/ZB)
	case b >= EB:
		return fmt.Sprintf("%.2fEB", b/EB)
	case b >= PB:
		return fmt.Sprintf("%.2fPB", b/PB)
	case b >= TB:
		return fmt.Sprintf("%.2fTB", b/TB)
	case b >= GB:
		return fmt.Sprintf("%.2fGB", b/GB)
	case b >= MB:
		return fmt.Sprintf("%.2fMB", b/MB)
	case b >= KB:
		return fmt.Sprintf("%.2fKB", b/KB)
	}
	return fmt.Sprintf("%.2fB", b)
}

const (
	_           = iota // ignore first value by assigning to blank identifier
	KB ByteSize = 1 << (10 * iota)
	MB
	GB
	TB
	PB
	EB
	ZB
	YB
)
