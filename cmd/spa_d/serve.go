package main

import (
	"context"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

func init() {
	mime.AddExtensionType(".js", "application/javascript")
	mime.AddExtensionType(".mjs", "application/javascript")
	mime.AddExtensionType(".cjs", "application/javascript")
	mime.AddExtensionType(".svg", "image/svg+xml")
}

func Must(err error) {
	if err != nil {
		panic(err)
	}
}

type server struct {
	cfg    Config
	logger zerolog.Logger
}

func (srv *server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	srv.handler(ctx, w, req)
}

func (srv *server) handler(ctx context.Context, w http.ResponseWriter, req *http.Request) {
	ctx, span := telemetry().tracer.Start(
		ctx, "spa_d.serve_asset",
		trace.WithAttributes(attribute.String("path", req.URL.Path)),
	)
	defer span.End()

	logger := srv.logger.With().Str("path", req.URL.Path).Logger()

	resourcePath := req.URL.Path
	// strip base url
	if srv.cfg.BaseURL != "" {
		if strings.HasPrefix(req.URL.Path, srv.cfg.BaseURL) {
			resourcePath = req.URL.Path[len(srv.cfg.BaseURL):]
		} else if req.URL.Path+"/" == srv.cfg.BaseURL {
			resourcePath = ""
		} else if !srv.cfg.AllowSkipBaseUrl {
			span.SetStatus(codes.Error, "base url missing")
			logger.Info().Int("status", http.StatusNotFound).Msg("not found - base url mismatch")
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
	}

	if resourcePath == "" {
		resourcePath = "index.html"
	}

	found, err := srv.findAndServeEncoded(ctx, resourcePath, w, req)

	if !found && err == nil {
		found, err = srv.importFallback(ctx, resourcePath, w, req)
	}

	if !found && err == nil {
		found, err = srv.fallback(ctx, w, req)
	}

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		logger.Err(err).Int("status", http.StatusInternalServerError).Msg("Error serving asset")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if !found {
		telemetry().not_found.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("path", req.URL.Path),
			))

		logger.Info().Int("status", http.StatusNotFound).Msg("not found")
		span.SetStatus(codes.Error, "not found")
		http.Error(w, "Not Found", http.StatusNotFound)
	}
	span.SetStatus(codes.Ok, "ok")
}

func (srv *server) fallback(ctx context.Context, w http.ResponseWriter, req *http.Request) (bool, error) {
	if srv.cfg.FallbackDisabled {
		return false, nil
	}

	if len(req.Header.Get("Accept")) != 0 &&
		!slices.ContainsFunc(
			req.Header.Values("Accept"),
			func(acp string) bool { return strings.HasPrefix(acp, "text/html") },
		) {
		return false, nil
	}

	for _, regex := range srv.cfg.NotFoundRegexs {
		if match, _ := regexp.MatchString(regex, req.URL.Path); match {
			return false, nil
		}
	}

	found, err := srv.findAndServeEncoded(ctx, "/index.html", w, req)
	if found {
		telemetry().fallbacks.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("path", req.URL.Path),
			))
	}

	return found, err
}

func (srv *server) findAndServeEncoded(ctx context.Context, resourcePath string, w http.ResponseWriter, req *http.Request) (bool, error) {
	encodings := []string{}

	if !srv.cfg.BrotliDisabled {
		encodings = append(encodings, "br")
	}

	if !srv.cfg.GzipDisabled {
		encodings = append(encodings, "gzip")
	}

	for _, encoding := range encodings {
		if slices.ContainsFunc(
			req.Header.Values("Accept-Encoding"),
			func(enc string) bool { return strings.HasPrefix(enc, encoding) },
		) {
			found, err := func() (bool, error) {
				ctx, span := telemetry().tracer.Start(
					ctx, "spa_d.lookup_"+encoding+"_asset",
					trace.WithAttributes(attribute.String("path", req.URL.Path)),
					trace.WithAttributes(attribute.String("encoding", encoding)),
				)
				defer span.End()

				ext := encoding
				if encoding == "gzip" {
					ext = "gz"
				}

				if file, ok, _ := srv.findFile(ctx, resourcePath+"."+ext); ok {
					defer file.Close()

					// set content type of unencrypted file
					w.Header().Set("Content-Encoding", encoding)
					ctype := srv.resolveMimeType(ctx, resourcePath)
					w.Header().Set("Content-Type", ctype)
					if encoding == "br" {
						telemetry().brotli_encrypted.Add(ctx, 1,
							metric.WithAttributes(
								attribute.String("path", req.URL.Path),
							))
					}
					if encoding == "gzip" {
						telemetry().gzip_encrypted.Add(ctx, 1,
							metric.WithAttributes(
								attribute.String("path", req.URL.Path),
							))
					}
					err := srv.serveContent(ctx, w, req, resourcePath, file)
					return err == nil, err
				}
				return false, nil
			}()
			if found || err != nil {
				return found, err
			}
		}
	}
	return srv.findAndServe(ctx, resourcePath, w, req)
}

func (srv *server) resolveMimeType(ctx context.Context, resourcePath string) string {
	ctype := mime.TypeByExtension(filepath.Ext(resourcePath))
	if ctype == "" {
		// find original resource and sniff content type
		org, ok, err := srv.findFile(ctx, resourcePath)
		Must(err) //wal already found
		defer org.Close()
		if ok {
			// read a chunk to decide between utf-8 text and binary
			var buf [512]byte
			n, _ := io.ReadFull(org, buf[:])
			ctype = http.DetectContentType(buf[:n])
		}
	}
	return ctype
}

func (srv *server) findAndServe(ctx context.Context, resourcePath string, w http.ResponseWriter, req *http.Request) (bool, error) {
	file, ok, err := srv.findFile(ctx, resourcePath)
	if err != nil {
		return false, err
	}
	if ok {
		defer file.Close()
		ctype := srv.resolveMimeType(ctx, resourcePath)
		w.Header().Set("Content-Type", ctype)
		err := srv.serveContent(ctx, w, req, resourcePath, file)
		return err == nil, err
	}
	return false, nil
}

func (srv *server) serveContent(ctx context.Context, w http.ResponseWriter, req *http.Request, name string, file *os.File) error {
	ctx, span := telemetry().tracer.Start(
		ctx, "spa_d.lserve_content",
	)
	defer span.End()
	logger := srv.logger.With().Str("path", req.URL.Path).Logger()
	srv.applyHeaders(w, name)
	info, err := file.Stat()
	Must(err)

	http.ServeContent(w, req, name, info.ModTime(), file)
	logger.Info().Int("status", http.StatusOK).Msg("asset served")
	return nil
}

func (srv *server) findFile(ctx context.Context, resourcePath string) (*os.File, bool, error) {
	ctx, span := telemetry().tracer.Start(
		ctx, "spa_d.lookup_asset",
		trace.WithAttributes(attribute.String("file", resourcePath)),
	)
	defer span.End()
	for _, rootDir := range srv.cfg.RootDirs {

		logger := srv.logger.With().Str("path", resourcePath).Logger()
		filePath := path.Join(rootDir, resourcePath)
		file, err := os.Open(filePath)
		var info os.FileInfo
		if err == nil {
			info, err = file.Stat()
		}
		if err == nil && !info.IsDir() {
			logger.Info().Str("file", filePath).Msg("asset found")
			return file, true, nil
		}
		if err != nil && !os.IsNotExist(err) {
			return nil, false, err
		}
		file.Close()
	}

	return nil, false, nil
}

func (srv *server) applyHeaders(
	w http.ResponseWriter,
	resourcePath string,
) {

	// path specific headers
	for rx, headers := range srv.cfg.HeadersPerPathRegex {
		if match, _ := regexp.MatchString(rx, resourcePath); match {
			for hdr, value := range headers {
				w.Header().Set(hdr, value)
			}
		}
	}

	// merge missing global headers
	for key, value := range srv.cfg.Headers {
		if _, ok := w.Header()[key]; !ok {
			w.Header().Set(key, value)
		}
	}

	// default cache control
	if _, ok := w.Header()["Cache-Control"]; !ok {
		if resourcePath != "/index.html" {
			// set imutable cache header
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			// set no cache - index.html may be ssr rendered
			w.Header().Set("Cache-Control", "no-cache")
		}
	}

}

var importFallbackMap sync.Map = sync.Map{}

// importFallbacks tries to serve JavaScript files that are not found by adding ".js" suffix
// srv is usefull if the SPA uses import statements  but internally omits the ".js" suffix (e.g. Vite projects)
func (srv *server) importFallback(ctx context.Context, resourcePath string, w http.ResponseWriter, req *http.Request) (bool, error) {
	if !strings.HasPrefix(srv.cfg.ImportFallbackRegexp, "disable") &&
		!strings.HasSuffix(resourcePath, ".mjs") &&
		!strings.HasSuffix(resourcePath, ".js") &&
		!strings.HasSuffix(resourcePath, ".cjs") {

		// check cache
		if val, ok := importFallbackMap.Load(resourcePath); ok {
			realPath := val.(string)
			return srv.findAndServeEncoded(ctx, realPath, w, req)
		}

		// check regexp if set
		if srv.cfg.ImportFallbackRegexp != "" {
			matched := false
			re, err := regexp.Compile(srv.cfg.ImportFallbackRegexp)
			if err != nil {
				return false, err
			}
			if re.MatchString(resourcePath) {
				matched = true
			}
			if !matched {
				return false, nil
			}
		}

		realPath := resourcePath + ".mjs"
		found, err := srv.findAndServeEncoded(ctx, realPath, w, req)

		if !found && err == nil {
			realPath = resourcePath + ".js"
			found, err = srv.findAndServeEncoded(ctx, realPath, w, req)
		}
		if !found && err == nil {
			realPath = resourcePath + ".cjs"
			found, err = srv.findAndServeEncoded(ctx, realPath, w, req)
		}

		if found {
			importFallbackMap.Store(resourcePath, realPath)
		}

		return found, err
	}
	return false, nil
}
