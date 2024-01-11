package main

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

type Config struct {
	// Port is the port to listen on.
	Port int `json:"port"`

	// LoggingLevel is the logging level.
	LoggingLevel string `json:"logging-level"`

	// JsonLogging is whether to log in json format.
	JsonLogging bool `json:"json-logging"`

	// RootDirs is the list of root directories to search for resources.
	RootDirs []string `json:"roots"`

	// Headers is the map of headers to add to responses.
	Headers map[string]string `json:"headers"`

	// HeadersPerPathRegex is the map of headers per path regex to add to responses.
	HeadersPerPathRegex map[string]map[string]string `json:"headers-per-regexp"`

	// NotFoundRegexs is the list of path regexs to return 404 instead of fallback html.
	NotFoundRegexs []string `json:"no-fallback-regexp"`

	// wheter to disable fallback to index.html
	FallbackDisabled bool `json:"fallback-disabled"`

	// gzip encoding disabled
	GzipDisabled bool `json:"gzip-disabled"`

	// brotli encoding disabled
	BrotliDisabled bool `json:"brotli-disabled"`

	// telemetry disabled
	TelemetryDisabled bool `json:"telemetry-disabled"`
}

func loadConfiguration() (cfg Config) {
	err := configureViper()
	cfg = Config{}
	err = viper.Unmarshal(&cfg)
	if err != nil {
		log.Fatal("Cannot read configuration")
	}
	return
}

func configureViper() error {
	viper.AddConfigPath("config")
	viper.SetConfigName("spa_base")
	viper.SetConfigType("yaml")
	setDefaults()

	viper.SetEnvKeyReplacer(strings.NewReplacer(`.`, `_`, `-`, `_`))
	viper.SetEnvPrefix("SPA_BASE")
	viper.AutomaticEnv()

	err := viper.ReadInConfig()
	return err
}

func setDefaults() {
	viper.SetDefault("port", 7105)
	viper.SetDefault("logging-level", "Trace")
	viper.SetDefault("json-logging", true)
	viper.SetDefault("roots", []string{"./public"})
	viper.SetDefault("headers", map[string]string{})
	viper.SetDefault("headers-per-regexp", map[string]map[string]string{})
	viper.SetDefault("not-found-regexp", []string{"(\\.js|\\.json|\\.mjs|\\.png|\\.jpe?g|\\.woff2)"})
	viper.SetDefault("resourceName", "spa_d")

}

func configureLogger(cfg Config) zerolog.Logger {
	lvl := strings.ToLower(cfg.LoggingLevel)
	var loglevel zerolog.Level
	switch lvl {
	case "trace":
		loglevel = zerolog.TraceLevel
	case "debug":
		loglevel = zerolog.DebugLevel
	case "information":
		loglevel = zerolog.InfoLevel
	case "warning":
		loglevel = zerolog.WarnLevel
	case "error":
		loglevel = zerolog.ErrorLevel
	default:
		loglevel = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(loglevel)
	l := zerolog.New(os.Stderr).With().Timestamp().Logger()
	if !cfg.JsonLogging {
		l = l.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}
	l.Info().
		Str("logging-level", cfg.LoggingLevel).
		Str("port", strconv.Itoa(cfg.Port)).
		Str("roots", strings.Join(cfg.RootDirs, ", ")).
		Msg("Configuration")

	return l
}
