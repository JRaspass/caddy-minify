package caddyminify

import (
	"net/http"
	"strings"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/html"
	"github.com/tdewolff/minify/v2/json"
	"github.com/tdewolff/minify/v2/svg"
)

func init() {
	caddy.RegisterModule(new(Middleware))
	httpcaddyfile.RegisterHandlerDirective("minify", parseCaddyfileHandler)
}

type Middleware struct{ minifier *minify.M }

func parseCaddyfileHandler(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	m := new(Middleware)
	return m, m.UnmarshalCaddyfile(h.Dispenser)
}

func (*Middleware) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.minify",
		New: func() caddy.Module { return new(Middleware) },
	}
}

func (m *Middleware) Provision(_ caddy.Context) error {
	m.minifier = minify.New()

	m.minifier.AddFunc("text/html", html.Minify)
	m.minifier.AddFunc("application/json", json.Minify)
	m.minifier.AddFunc("image/svg+xml", svg.Minify)

	return nil
}

func (m *Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	fw := &responseMinifier{
		ResponseWriterWrapper: &caddyhttp.ResponseWriterWrapper{ResponseWriter: w},
		minifier:              m.minifier,
	}

	return next.ServeHTTP(fw, r)
}

func (*Middleware) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.Next() // Consume directive name.

	// There should be no more arguments.
	if d.NextArg() {
		return d.ArgErr()
	}

	return nil
}

type responseMinifier struct {
	*caddyhttp.ResponseWriterWrapper
	minifier *minify.M
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
		writer := fw.minifier.Writer(mediatype, fw.ResponseWriter)

		defer writer.Close()

		return writer.Write(d)
	}

	return fw.ResponseWriter.Write(d)
}

// Interface guards
var (
	_ caddy.Module                = (*Middleware)(nil)
	_ caddy.Provisioner           = (*Middleware)(nil)
	_ caddyhttp.MiddlewareHandler = (*Middleware)(nil)
	_ caddyfile.Unmarshaler       = (*Middleware)(nil)
	_ http.ResponseWriter         = (*responseMinifier)(nil)
)
