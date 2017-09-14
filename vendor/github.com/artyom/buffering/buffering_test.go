package buffering

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestHandler(t *testing.T) {
	payload := "Hello, gophers!\n"
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(payload))
	h := Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch f, ok := r.Body.(*os.File); {
		case ok:
			t.Logf("r.Body is a file: %q", f.Name())
		default:
			t.Logf("r.Body is not an *os.File: %#v", r.Body)
		}
		buf, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("reading r.Body: %v", err)
		}
		got := string(buf)
		t.Logf("read payload: %q", got)
		if got != payload {
			t.Fatalf("got wrong payload:\n%q\nwant:\n%q", got, payload)
		}
	}), WithBufSize(1024))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if got, want := w.Code, http.StatusOK; got != want {
		t.Fatalf("got code %d, want %d", got, want)
	}
	h.(*handler).maxSize = 5
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if got, want := w.Code, http.StatusRequestEntityTooLarge; got != want {
		t.Fatalf("got code %d, want %d", got, want)
	}
}
