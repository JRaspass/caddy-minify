package caddyminify

import (
	"bytes"
	"net/http"
	"strings"
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

type responseMinifier struct {
	*caddyhttp.ResponseWriterWrapper
	handler *Handler
}

func (fw *responseMinifier) WriteHeader(status int) {
	// we don't know the length after replacements since
	// we're not buffering it all to find out
	fw.Header().Del("Content-Length")

	fw.ResponseWriterWrapper.WriteHeader(status)
}

func (fw *responseMinifier) Write(d []byte) (int, error) {
	var mediatype = fw.ResponseWriter.Header().Get("Content-Type")

	if strings.HasPrefix(mediatype, "text/html") {
		writer := fw.handler.minifier.Writer(mediatype, fw.ResponseWriter)

		defer writer.Close()

		return writer.Write(d)
	}

	return fw.ResponseWriter.Write(d)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	fw := &responseMinifier{
		ResponseWriterWrapper: &caddyhttp.ResponseWriterWrapper{ResponseWriter: w},
		handler:               h,
	}

	return next.ServeHTTP(fw, r)
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
