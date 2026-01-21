package main

import (
	"context"
	"os"
	"path"
	"runtime"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
)

type MainTestSuite struct {
	suite.Suite
	cfg Config
}

func TestMainTestSuite(t *testing.T) {
	suite.Run(t, new(MainTestSuite))
}

func (suite *MainTestSuite) SetupTest() {
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := path.Join(path.Dir(filename), "test/data")
	processCtx = context.Background()

	suite.cfg = Config{
		// Use a random high port or 0 to let OS choose (though server struct needs port for Addr)
		// Assuming 0 works for binding but we might not know which one is picked easily without callback.
		// Using a likely free port for test purposes.
		Port:                0, // 0 lets net/http pick a random port
		RootDirs:            []string{projectRoot},
		Headers:             map[string]string{},
		HeadersPerPathRegex: map[string]map[string]string{},
		NotFoundRegexs:      []string{},
		TelemetryDisabled:   true,
	}
}

func (suite *MainTestSuite) Test_Run_StartsServerAndShutsDownOnContextCancel() {
	os.Setenv("SPA_BASE_LOGGING_LEVEL", "1024") // invoke fallback path
	os.Setenv("SPA_BASE_JSON_LOGGING", "false")
	os.Setenv("OTEL_TRACES_EXPORTER", "none")
	os.Setenv("OTEL_METRICS_EXPORTER", "none")
	defer os.Unsetenv("SPA_BASE_LOGGING_LEVEL")
	defer os.Unsetenv("SPA_BASE_JSON_LOGGING")

	ctx, cancel := context.WithCancel(context.Background())
	processCtx = ctx

	done := make(chan struct{})
	go func() {
		main()
		close(done)
	}()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Trigger shutdown via context
	cancel()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		suite.Fail("Run did not return after context cancellation")
	}
}

func (suite *MainTestSuite) Test_Run_StartsServerAndShutsDownOnSignal() {
	ctx := context.Background()
	logger := zerolog.New(os.Stdout)
	cfg := suite.cfg

	done := make(chan struct{})

	signalChannel := make(chan os.Signal, 2)
	go func() {
		run(ctx, cfg, logger, signalChannel)
		close(done)
	}()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Trigger shutdown via signal
	signalChannel <- os.Interrupt

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		suite.Fail("Run did not return after context cancellation")
	}
}

func (suite *MainTestSuite) Test_Run_PortZero_BindsSuccessfully() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logger := zerolog.New(os.Stdout)
	cfg := suite.cfg
	cfg.Port = 0 // Ask for random port

	done := make(chan struct{})
	signalChannel := make(chan os.Signal, 2)
	go func() {
		run(ctx, cfg, logger, signalChannel)
		close(done)
	}()

	// Wait for server to be likely up
	time.Sleep(100 * time.Millisecond)

	// Verify we can just close it
	cancel()
	<-done
}

func (suite *MainTestSuite) Test_Run_WithTelemetry_Disabled() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logger := zerolog.New(os.Stdout)
	cfg := suite.cfg
	cfg.TelemetryDisabled = true

	signalChannel := make(chan os.Signal, 2)
	go func() {
		run(ctx, cfg, logger, signalChannel)
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()
}
