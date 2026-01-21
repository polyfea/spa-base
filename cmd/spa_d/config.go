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
	Port int `mapstructure:"port"`

	// LoggingLevel is the logging level.
	LoggingLevel string `mapstructure:"logging-level"`

	// JsonLogging is whether to log in json format.
	JsonLogging bool `mapstructure:"json-logging"`

	// BaseURL is the base url to use for the server.
	// All file paths will be resolved as if relative to BaseURL.
	BaseURL string `mapstructure:"base-url"`

	// If enabled and request path does not starts with base url
	// then the file will be searched in using the request path as is.
	AllowSkipBaseUrl bool `mapstructure:"allow-skip-base-url"`

	// RootDirs is the list of root directories to search for resources.
	RootDirs []string `mapstructure:"roots"`

	// Headers is the map of headers to add to responses.
	Headers map[string]string `mapstructure:"headers"`

	// HeadersPerPathRegex is the map of headers per path regex to add to responses.
	HeadersPerPathRegex map[string]map[string]string `mapstructure:"headers-per-regexp"`

	// NotFoundRegexs is the list of path regexs to return 404 instead of fallback html.
	NotFoundRegexs []string `mapstructure:"no-fallback-regexp"`

	// wheter to disable fallback to index.html
	FallbackDisabled bool `mapstructure:"fallback-disabled"`

	// gzip encoding disabled
	GzipDisabled bool `mapstructure:"gzip-disabled"`

	// brotli encoding disabled
	BrotliDisabled bool `mapstructure:"brotli-disabled"`

	// telemetry disabled
	TelemetryDisabled bool `mapstructure:"telemetry-disabled"`

	// If set to disable then no fallbacks for imports will be attempted
	// otherwise shall be set to valid regexp to check which paths to attempt import fallbacks for.
	// If empty then all paths are attempted.
	ImportFallbackRegexp string `mapstructure:"import-fallback-regexp"`
}

func loadConfiguration() (cfg Config) {
	err := configureViper()
	cfg = Config{}
	err = viper.Unmarshal(&cfg)
	Must(err)
	return
}

func configureViper() error {
	viper.AddConfigPath("config")
	viper.SetConfigName("spa-base")
	viper.SetConfigType("yaml")
	setDefaults()

	viper.SetEnvKeyReplacer(strings.NewReplacer(`.`, `_`, `-`, `_`))
	viper.SetEnvPrefix("SPA_BASE")
	viper.AutomaticEnv()
	err := viper.ReadInConfig()
	if _, ok := err.(viper.ConfigFileNotFoundError); ok {
		log.Println("No configuration file found, using defaults")
		err = nil
	}
	return err

}

func setDefaults() {
	viper.SetDefault("port", 7105)
	viper.SetDefault("base-url", "/")
	viper.SetDefault("allow-skip-base-url", false)
	viper.SetDefault("logging-level", "info")
	viper.SetDefault("json-logging", true)
	viper.SetDefault("roots", []string{"./public"})
	viper.SetDefault("headers", map[string]string{})
	viper.SetDefault("headers-per-regexp", map[string]map[string]string{})
	viper.SetDefault("not-found-regexp", []string{"(\\.js|\\.json|\\.mjs|\\.png|\\.jpe?g|\\.woff2)"})
}

func configureLogger(cfg Config) zerolog.Logger {
	lvl := strings.ToLower(cfg.LoggingLevel)
	loglevel, err := zerolog.ParseLevel(lvl)
	if err != nil {
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
		Str("base-url", cfg.BaseURL).
		Bool("allow-skip-base-url", cfg.AllowSkipBaseUrl).
		Msg("Configuration")

	return l
}
