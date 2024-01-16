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

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

type server struct {
	cfg    Config
	logger zerolog.Logger
}

func (this *server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	this.handler(ctx, w, req)
}

func (this *server) handler(ctx context.Context, w http.ResponseWriter, req *http.Request) {
	ctx, span := telemetry().tracer.Start(
		ctx, "spa_d.serve_asset",
		trace.WithAttributes(attribute.String("path", req.URL.Path)),
	)
	defer span.End()

	logger := this.logger.With().Str("path", req.URL.Path).Logger()

	// strip base url
	if this.cfg.BaseURL != "" {
		if !strings.HasPrefix(req.URL.Path, this.cfg.BaseURL) {
			logger.Info().Int("status", http.StatusNotFound).Msg("not found - base url mismatch")
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		req.URL.Path = req.URL.Path[len(this.cfg.BaseURL):]
	}

	resourcePath := req.URL.Path
	if resourcePath == "" {
		resourcePath = "index.html"
	}

	found, err := this.findAndServeEncoded(ctx, resourcePath, w, req)

	if !found && err == nil {
		found, err = this.fallback(ctx, w, req)
	}

	if err != nil {
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
		http.Error(w, "Not Found", http.StatusNotFound)
	}
}

func (this *server) fallback(ctx context.Context, w http.ResponseWriter, req *http.Request) (bool, error) {
	if this.cfg.FallbackDisabled {
		return false, nil
	}

	if len(req.Header.Get("Accept")) != 0 &&
		!slices.ContainsFunc(
			req.Header.Values("Accept"),
			func(acp string) bool { return strings.HasPrefix(acp, "text/html") },
		) {
		return false, nil
	}

	for _, regex := range this.cfg.NotFoundRegexs {
		if match, _ := regexp.MatchString(regex, req.URL.Path); match {
			return false, nil
		}
	}

	found, err := this.findAndServeEncoded(ctx, "/index.html", w, req)
	if found {
		telemetry().fallbacks.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("path", req.URL.Path),
			))
	}

	return found, err
}

func (this *server) findAndServeEncoded(ctx context.Context, resourcePath string, w http.ResponseWriter, req *http.Request) (bool, error) {
	encodings := []string{}

	if !this.cfg.BrotliDisabled {
		encodings = append(encodings, "br")
	}

	if !this.cfg.GzipDisabled {
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

				if file, ok, _ := this.findFile(ctx, resourcePath+"."+ext); ok {
					defer file.Close()

					// set content type of unencrypted file
					w.Header().Set("Content-Encoding", encoding)
					ctype := mime.TypeByExtension(filepath.Ext(resourcePath))
					if ctype == "" {
						// find original resource and sniff content type
						org, ok, err := this.findFile(ctx, resourcePath)
						defer org.Close()
						if err != nil {
							return false, err
						}
						if ok {
							// read a chunk to decide between utf-8 text and binary
							var buf [512]byte
							n, _ := io.ReadFull(org, buf[:])
							ctype = http.DetectContentType(buf[:n])
						}
					}

					if ctype == "" {
						// fallback to binary if content type could not be detected
						ctype = "application/octet-stream"
					}

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
					err := this.serveContent(ctx, w, req, resourcePath, file)
					return err == nil, err
				}
				return false, nil
			}()
			if found || err != nil {
				return found, err
			}
		}
	}
	return this.findAndServe(ctx, resourcePath, w, req)
}

func (this *server) findAndServe(ctx context.Context, resourcePath string, w http.ResponseWriter, req *http.Request) (bool, error) {
	file, ok, err := this.findFile(ctx, resourcePath)
	if err != nil {
		return false, err
	}
	if ok {
		defer file.Close()
		err := this.serveContent(ctx, w, req, resourcePath, file)
		return err == nil, err
	}
	return false, nil
}

func (this *server) serveContent(ctx context.Context, w http.ResponseWriter, req *http.Request, name string, file *os.File) error {
	logger := this.logger.With().Str("path", req.URL.Path).Logger()
	this.applyHeaders(ctx, w, req, name)
	info, err := file.Stat()
	if err != nil {
		logger.Err(err).Int("status", http.StatusInternalServerError).Msg("Error getting file info")
		return err
	}

	http.ServeContent(w, req, name, info.ModTime(), file)
	logger.Info().Int("status", http.StatusOK).Msg("asset served")
	return nil
}

func (this *server) findFile(ctx context.Context, resourcePath string) (*os.File, bool, error) {
	ctx, span := telemetry().tracer.Start(
		ctx, "spa_d.lookup_asset",
		trace.WithAttributes(attribute.String("file", resourcePath)),
	)
	defer span.End()

	for _, rootDir := range this.cfg.RootDirs {
		logger := this.logger.With().Str("path", resourcePath).Logger()
		filePath := path.Join(rootDir, resourcePath)
		file, err := os.Open(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, false, nil
			}
			logger.Err(err).Msg("Error opening file")
			return nil, false, err
		}
		info, err := file.Stat()
		if err != nil {
			file.Close()
			logger.Err(err).Msg("Error getting file info")
			return nil, false, err
		}

		if info.IsDir() {
			file.Close()
			return nil, false, nil
		}
		return file, true, nil
	}

	return nil, false, nil
}

func (this *server) applyHeaders(
	ctx context.Context,
	w http.ResponseWriter,
	req *http.Request,
	resourcePath string,
) {

	// path specific headers
	for rx, headers := range this.cfg.HeadersPerPathRegex {
		if match, _ := regexp.MatchString(rx, resourcePath); match {
			for hdr, value := range headers {
				w.Header().Set(hdr, value)
			}
		}
	}

	// merge missing global headers
	for key, value := range this.cfg.Headers {
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
