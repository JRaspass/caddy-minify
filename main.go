package caddyminify

import (
	"bytes"
	"net/http"
	"sync"

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

var bufPool = sync.Pool{New: func() any { return new(bytes.Buffer) }}

func (m *Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	// Get a buffer to hold the response body.
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)

	// Set up the response recorder.
	shouldBuf := func(int, http.Header) bool { return true }
	rec := caddyhttp.NewResponseRecorder(w, buf, shouldBuf)

	// Collect the response from upstream.
	if err := next.ServeHTTP(rec, r); err != nil {
		return err
	}

	// Early-exit if the body isn't HTML.
	mediaType := rec.Header().Get("Content-Type")
	_, params, minifier := m.minifier.Match(mediaType)
	if minifier == nil {
		return rec.WriteResponse()
	}

	// Minify the body.
	var result bytes.Buffer
	if err := minifier(m.minifier, &result, buf, params); err != nil {
		return err
	}

	// Write out the shorter body.
	w.Header().Del("Content-Length")
	w.WriteHeader(rec.Status())
	_, err := w.Write(result.Bytes())
	return err
}

func (*Middleware) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.Next() // Consume directive name.

	// There should be no more arguments.
	if d.NextArg() {
		return d.ArgErr()
	}

	return nil
}

// Interface guards
var (
	_ caddy.Module                = (*Middleware)(nil)
	_ caddy.Provisioner           = (*Middleware)(nil)
	_ caddyhttp.MiddlewareHandler = (*Middleware)(nil)
	_ caddyfile.Unmarshaler       = (*Middleware)(nil)
)
