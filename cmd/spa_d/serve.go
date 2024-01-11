package main

import (
	"context"
	"net/http"
	"os"
	"path"
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
	resourcePath := req.URL.Path
	if resourcePath == "/" {
		resourcePath = "/index.html"
	}

	found, err := this.findAndServeEncoded(ctx, resourcePath, w, req)

	if found || err != nil {
		return
	}

	if found := this.fallback(ctx, w, req); !found {
		telemetry().not_found.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("path", req.URL.Path),
			))

		logger.Info().Int("status", http.StatusNotFound).Msg("not found")
		http.Error(w, "Fallback to index.html -  Not Found", http.StatusNotFound)
	}

}

func (this *server) fallback(ctx context.Context, w http.ResponseWriter, req *http.Request) bool {
	if this.cfg.FallbackDisabled {
		return false
	}

	if len(req.Header.Get("Accept")) != 0 &&
		!slices.ContainsFunc(
			req.Header.Values("Accept"),
			func(acp string) bool { return strings.HasPrefix(acp, "text/html") },
		) {
		return false
	}

	for _, regex := range this.cfg.NotFoundRegexs {
		if match, _ := regexp.MatchString(regex, req.URL.Path); match {
			return false
		}
	}

	found, _ := this.findAndServeEncoded(ctx, "/index.html", w, req)
	if found {
		telemetry().fallbacks.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("path", req.URL.Path),
			))
	}

	return found
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

				found, err := this.findAndServe(ctx, resourcePath+"."+ext, w, req)
				if err != nil {
					return false, err
				}

				if found {
					w.Header().Set("Content-Encoding", encoding)
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
					return true, nil
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
	for _, rootDir := range this.cfg.RootDirs {
		filePath := path.Join(rootDir, resourcePath)
		found, err := this.lookupFile(ctx, filePath, resourcePath, w, req)
		if found || err != nil {
			return found, err
		}
	}
	return false, nil
}

func (this *server) lookupFile(
	ctx context.Context,
	filePath string,
	resourcePath string,
	w http.ResponseWriter,
	req *http.Request,
) (bool, error) {
	ctx, span := telemetry().tracer.Start(
		ctx, "spa_d.lookup_asset",
		trace.WithAttributes(attribute.String("path", req.URL.Path)),
		trace.WithAttributes(attribute.String("file", filePath)),
	)
	defer span.End()

	logger := this.logger.With().Str("path", req.URL.Path).Logger()
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		logger.Err(err).Int("status", http.StatusInternalServerError).Msg("Error opening file")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return false, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		logger.Err(err).Int("status", http.StatusInternalServerError).Msg("Error getting file info")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return false, err
	}

	if info.IsDir() {
		return false, nil
	}

	if resourcePath != "/index.html" {
		// set imutable cache header
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		// set no cache - index.html may be ssr rendered
		w.Header().Set("Cache-Control", "no-cache")
	}

	for key, value := range this.cfg.Headers {
		w.Header().Set(key, value)
	}

	for rx, headers := range this.cfg.HeadersPerPathRegex {
		if match, _ := regexp.MatchString(rx, resourcePath); match {
			for hdr, value := range headers {
				w.Header().Set(hdr, value)
			}
		}
	}

	telemetry().resources_served.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("path", req.URL.Path),
			attribute.String("file", filePath),
		))
	http.ServeContent(w, req, resourcePath, info.ModTime(), file)
	logger.Info().Int("status", http.StatusOK).Msg("asset served")
	return true, nil
}
