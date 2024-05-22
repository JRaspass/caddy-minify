package caddyminify

import (
	"bytes"
	"net/http"
	"strconv"
	"sync"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/html"
	"github.com/tdewolff/minify/v2/json"
	"github.com/tdewolff/minify/v2/svg"
)

var bufferPool = sync.Pool{New: func() any { return new(bytes.Buffer) }}

type Handler struct{ minifier *minify.M }

func init() {
	caddy.RegisterModule(new(Handler))
	httpcaddyfile.RegisterHandlerDirective("minify", setup)
}

func setup(_ httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	return new(Handler), nil
}

func (*Handler) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.minify",
		New: func() caddy.Module { return new(Handler) },
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	resBuffer := bufferPool.Get().(*bytes.Buffer)
	resBuffer.Reset()
	defer bufferPool.Put(resBuffer)

	shouldBuffer := func(_ int, _ http.Header) bool { return true }
	recorder := caddyhttp.NewResponseRecorder(w, resBuffer, shouldBuffer)

	err := next.ServeHTTP(recorder, r)
	if err != nil {
		return err
	}

	if resBuffer.Len() < 1 {
		w.WriteHeader(recorder.Status())
		return nil
	}

	if recorder.Header().Get("Content-Encoding") != "" {
		w.WriteHeader(recorder.Status())
		_, err = w.Write(resBuffer.Bytes())
		return err
	}

	result := &bytes.Buffer{}
	contentType := recorder.Header().Get("Content-Type")

	err = h.minifier.Minify(contentType, result, resBuffer)
	if err != nil {
		w.WriteHeader(recorder.Status())
		_, err = w.Write(resBuffer.Bytes())
		return err
	}

	w.Header().Set("Content-Length", strconv.Itoa(result.Len()))
	w.WriteHeader(recorder.Status())
	_, err = w.Write(result.Bytes())

	return err
}

func (h *Handler) Provision(_ caddy.Context) error {
	h.minifier = minify.New()

	h.minifier.AddFunc("text/html", html.Minify)
	h.minifier.AddFunc("application/json", json.Minify)
	h.minifier.AddFunc("image/svg+xml", svg.Minify)

	return nil
}

var _ caddy.Provisioner = (*Handler)(nil)
var _ caddyhttp.MiddlewareHandler = (*Handler)(nil)
