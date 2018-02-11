// Command pdfsvc is a small wrapper around wkhtmltopdf command to expose it as
// a http service
package main

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/net/html/charset"

	"github.com/artyom/autoflags"
	"github.com/artyom/buffering"
	"github.com/artyom/exitstatus"
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
		select {
		case <-ctx.Done():
			log.Print(exitstatus.Reason(err), " / ", exitstatus.Stats(cmd.ProcessState), ", ", ctx.Err())
		default:
			log.Print(exitstatus.Reason(err), " / ", exitstatus.Stats(cmd.ProcessState))
		}
	}
	if err != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		return nil, err
	}
	return bytes.NewReader(out), nil
}
