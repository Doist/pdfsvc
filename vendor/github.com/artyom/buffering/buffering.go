// Package buffering provides http.Handler that buffers bodies of http requests
// to on-disk files before passing this request to child handler.
//
// This may be useful if request is proxied to some backend that should read
// request as fast as possible and should not be bound by slow link of the
// client (i.e. Python backend).
package buffering

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

// Option modifies handler logic
type Option func(*handler)

// WithMaxSize configures Handler to limit max allowed request body size. If
// request body exceeds given size, Handler replies with 413 Payload Too Large.
func WithMaxSize(size int64) Option { return func(h *handler) { h.maxSize = size } }

// WithDir configures Handler to use given directory to store temporary files.
// It doesn't check whether directory is accessible.
func WithDir(dir string) Option { return func(h *handler) { h.dir = dir } }

// WithBufSize configures Handler to user in-memory buffers for payloads which
// do not exceed given size. Only bodies with Content-Length header are handled
// using in-memory buffers, chunked payloads fall back to files.
func WithBufSize(size int) Option { return func(h *handler) { h.bufSize = int64(size) } }

// WithFileLimit limits max files Handler is allowed to keep opened. This may be
// useful to limit number of file descriptors and OS threads, since file IO in
// Go consume real threads. Optional wait duration specifies how long request
// would be waiting in the queue for available slot until giving up. If 0, wait
// time is not limited (until request is canceled).
func WithFileLimit(maxFiles int, wait time.Duration) Option {
	return func(h *handler) {
		if maxFiles < 1 {
			return
		}
		h.gate = make(chan struct{}, maxFiles)
		h.wait = wait
	}
}

// Handler returns http.Handler wrapping original one; this handler first reads
// request body storing it to in-memory buffer or temporary file, then calls
// original handler to process request body directly from buffer or file.
//
// If no options are given, Handler defaults to creating files in default
// temporary directory, has no max payload or concurrency limit and uses
// in-memory buffers for payloads not exceeding DefaultBufSize.
func Handler(h http.Handler, opts ...Option) http.Handler {
	bh := &handler{Handler: h, bufSize: DefaultBufSize}
	for _, opt := range opts {
		opt(bh)
	}
	return bh
}

// DefaultBufSize sets default max request body size which is buffered using
// in-memory buffers for newly created Handler. Use WithBufSize option to adjust
// Handler configuration.
const DefaultBufSize = 32 * 1024 // 32K

type handler struct {
	http.Handler
	dir     string // directory for temp files, if empty, default one is used
	maxSize int64  // max allowed body size, unlimited if zero
	bufSize int64  // use buffers for payloads not exceeding this size

	gate chan struct{} // limit concurrent usage of open files
	wait time.Duration // queue wait time
}

func logFunc(r *http.Request) func(format string, v ...interface{}) {
	if r == nil {
		return func(string, ...interface{}) {}
	}
	srv, ok := r.Context().Value(http.ServerContextKey).(*http.Server)
	if ok && srv.ErrorLog != nil {
		return srv.ErrorLog.Printf
	}
	return log.Printf
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.ContentLength == 0 {
		h.Handler.ServeHTTP(w, r)
		return
	}
	if h.maxSize > 0 && r.ContentLength > h.maxSize {
		wErr(w, http.StatusRequestEntityTooLarge)
		return
	}
	var body io.ReadCloser
	var dst io.Writer
	logf := logFunc(r)
	switch {
	case h.bufSize > 0 && r.ContentLength > 0 && r.ContentLength <= h.bufSize:
		buf := bytes.NewBuffer(make([]byte, 0, int(r.ContentLength)))
		body, dst = ioutil.NopCloser(buf), buf
	default:
		if h.gate != nil {
			ctx := r.Context()
			var cancel func()
			if h.wait > 0 {
				ctx, cancel = context.WithTimeout(ctx, h.wait)
				defer cancel()
			}
			select {
			case h.gate <- struct{}{}:
				defer func() { <-h.gate }()
			case <-ctx.Done():
				wErr(w, http.StatusServiceUnavailable)
				return
			}
		}
		f, err := ioutil.TempFile(h.dir, ".request-")
		if err != nil {
			logf("buffering.Handler: temp file create: %v", err)
			wErr(w, http.StatusInternalServerError)
			return
		}
		defer f.Close()
		_ = os.Remove(f.Name())
		body, dst = f, f
	}
	switch max := h.maxSize; {
	case max > 0:
		if r.ContentLength > 0 {
			max = r.ContentLength
		}
		if _, err := io.CopyN(dst, r.Body, max); err != nil && err != io.EOF {
			logf("buffering.Handler: body copy: %v", err)
			wErr(w, http.StatusInternalServerError)
			return
		}
		// if read past body end succeeds, then it's not completely
		// consumed and client is over limit
		if n, err := io.CopyN(ioutil.Discard, r.Body, 32); err != io.EOF || n > 0 {
			wErr(w, http.StatusRequestEntityTooLarge)
			return
		}
	default:
		if _, err := io.Copy(dst, r.Body); err != nil {
			logf("buffering.Handler: body copy: %v", err)
			wErr(w, http.StatusInternalServerError)
			return
		}
	}
	if f, ok := dst.(*os.File); ok {
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			logf("buffering.Handler: file seek: %v", err)
			wErr(w, http.StatusInternalServerError)
			return
		}
	}
	r2 := new(http.Request)
	*r2 = *r
	r2.Body = body
	h.Handler.ServeHTTP(w, r2)
}

func wErr(w http.ResponseWriter, code int) { http.Error(w, http.StatusText(code), code) }
