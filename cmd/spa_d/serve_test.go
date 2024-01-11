package main

import (
	"context"
	_ "embed"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"runtime"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
)

//go:embed test/data/testfile.json
var testfile_json string

//go:embed test/data/index.html
var index_html string

//go:embed test/data/prebr.js
var prebr_js string

//go:embed test/data/prebr.js.br
var prebr_js_br string

//go:embed test/data/prebr.js.gz
var prebr_js_gz string

type ServeTestSuite struct {
	suite.Suite
	testfile_json string
	cfg           Config
}

func TestServeTestSuite(t *testing.T) {
	suite.Run(t, new(ServeTestSuite))
}

func (suite *ServeTestSuite) SetupTest() {

	zerolog.SetGlobalLevel(zerolog.Disabled)
	suite.testfile_json = testfile_json

	_, filename, _, _ := runtime.Caller(0)
	projectRoot := path.Join(path.Dir(filename), "test/data")

	suite.cfg = Config{
		Port:                7105,
		RootDirs:            []string{projectRoot},
		Headers:             map[string]string{},
		HeadersPerPathRegex: map[string]map[string]string{},
		NotFoundRegexs:      []string{},
		ResourceName:        "spa_d",
		LoggingLevel:        "info",
		JsonLogging:         false,
	}

}

func (suite *ServeTestSuite) Test_File_exists_Then_OK_With_Content() {

	// given
	sut := &server{
		cfg:    suite.cfg,
		logger: zerolog.New(os.Stdout),
	}

	req, err := http.NewRequest("GET", "/testfile.json", nil)

	suite.Nil(err)

	rr := httptest.NewRecorder()

	// when
	sut.handler(context.Background(), rr, req)

	// then

	suite.Equal(http.StatusOK, rr.Code)
	suite.Equal(testfile_json, rr.Body.String())

}

func (suite *ServeTestSuite) Test_File_not_exists_Then_Fallback_To_Index() {

	// given
	sut := &server{
		cfg:    suite.cfg,
		logger: zerolog.New(os.Stdout),
	}

	req, err := http.NewRequest("GET", "/nonexistent", nil)
	suite.Nil(err)

	rr := httptest.NewRecorder()

	// when

	sut.handler(context.Background(), rr, req)

	// then
	suite.Equal(http.StatusOK, rr.Code)
	suite.Equal(index_html, rr.Body.String())

}

func (suite *ServeTestSuite) Test_File_not_exists_and_fallback_disabled_Then_NotFound() {

	// given
	cfg := suite.cfg
	cfg.FallbackDisabled = true
	sut := &server{
		cfg:    cfg,
		logger: zerolog.New(os.Stdout),
	}

	req, err := http.NewRequest("GET", "/nonexistent", nil)
	suite.Nil(err)
	req.Header.Set("Accept", "text/html")

	rr := httptest.NewRecorder()

	// when

	sut.handler(context.Background(), rr, req)

	// then
	suite.Equal(http.StatusNotFound, rr.Code)
}

func (suite *ServeTestSuite) Test_File_not_exists_and_excluded_Then_NotFound() {

	// given
	cfg := suite.cfg
	cfg.NotFoundRegexs = []string{"\\.json"}
	sut := &server{
		cfg:    cfg,
		logger: zerolog.New(os.Stdout),
	}

	req, err := http.NewRequest("GET", "/nonexistent.json", nil)
	suite.Nil(err)

	rr := httptest.NewRecorder()

	// when

	sut.handler(context.Background(), rr, req)

	// then
	suite.Equal(http.StatusNotFound, rr.Code)
}

func (suite *ServeTestSuite) Test_File_not_exists_and_not_accepts_html_Then_NotFound() {

	// given
	cfg := suite.cfg
	cfg.NotFoundRegexs = []string{"\\.json"}
	sut := &server{
		cfg:    cfg,
		logger: zerolog.New(os.Stdout),
	}

	req, err := http.NewRequest("GET", "/nonexistent.json", nil)
	req.Header.Set("Accept", "application/json")
	suite.Nil(err)

	rr := httptest.NewRecorder()

	// when

	sut.handler(context.Background(), rr, req)

	// then
	suite.Equal(http.StatusNotFound, rr.Code)
}

func (suite *ServeTestSuite) Test_File_precompressed_br_Then_OK_and_encoded() {

	// given
	sut := &server{
		cfg:    suite.cfg,
		logger: zerolog.New(os.Stdout),
	}

	req, err := http.NewRequest("GET", "/prebr.js", nil)
	req.Header.Set("Accept-Encoding", "br, gzip")
	suite.Nil(err)

	rr := httptest.NewRecorder()

	// when
	sut.handler(context.Background(), rr, req)

	// then
	suite.Equal(http.StatusOK, rr.Code)
	suite.Equal("br", rr.Header().Get("Content-Encoding"))
	suite.Equal(prebr_js_br, rr.Body.String())
}

func (suite *ServeTestSuite) Test_File_precompressed_br_disabled_Then_OK_and_not_encoded() {

	// given
	cfg := suite.cfg
	cfg.BrotliDisabled = true
	sut := &server{
		cfg:    cfg,
		logger: zerolog.New(os.Stdout),
	}

	req, err := http.NewRequest("GET", "/prebr.js", nil)
	req.Header.Set("Accept-Encoding", "br")
	suite.Nil(err)

	rr := httptest.NewRecorder()

	// when
	sut.handler(context.Background(), rr, req)

	// then
	suite.Equal(http.StatusOK, rr.Code)
	suite.Equal("", rr.Header().Get("Content-Encoding"))
	suite.Equal(prebr_js, rr.Body.String())
}

func (suite *ServeTestSuite) Test_File_precompressed_gz_Then_OK_and_encoded() {

	// given
	sut := &server{
		cfg:    suite.cfg,
		logger: zerolog.New(os.Stdout),
	}

	req, err := http.NewRequest("GET", "/prebr.js", nil)
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	suite.Nil(err)

	rr := httptest.NewRecorder()

	// when
	sut.handler(context.Background(), rr, req)

	// then
	suite.Equal(http.StatusOK, rr.Code)
	suite.Equal("gzip", rr.Header().Get("Content-Encoding"))
	suite.Equal(prebr_js_gz, rr.Body.String())
}

func (suite *ServeTestSuite) Test_File_precompressed_gzip_disabled_Then_OK_and_not_encoded() {

	// given
	cfg := suite.cfg
	cfg.GzipDisabled = true
	sut := &server{
		cfg:    cfg,
		logger: zerolog.New(os.Stdout),
	}

	req, err := http.NewRequest("GET", "/prebr.js", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	suite.Nil(err)

	rr := httptest.NewRecorder()

	// when
	sut.handler(context.Background(), rr, req)

	// then
	suite.Equal(http.StatusOK, rr.Code)
	suite.Equal("", rr.Header().Get("Content-Encoding"))
	suite.Equal(prebr_js, rr.Body.String())
}

func (suite *ServeTestSuite) Test_File_exist_Then_cache_immutable() {

	// given
	sut := &server{
		cfg:    suite.cfg,
		logger: zerolog.New(os.Stdout),
	}

	req, err := http.NewRequest("GET", "/testfile.json", nil)
	suite.Nil(err)

	rr := httptest.NewRecorder()

	// when
	sut.handler(context.Background(), rr, req)

	// then
	suite.Equal(http.StatusOK, rr.Code)
	suite.Equal("public, max-age=31536000, immutable", rr.Header().Get("Cache-Control"))
}

func (suite *ServeTestSuite) Test_Index_Then_no_cache() {

	// given
	sut := &server{
		cfg:    suite.cfg,
		logger: zerolog.New(os.Stdout),
	}

	req, err := http.NewRequest("GET", "/", nil)
	suite.Nil(err)

	rr := httptest.NewRecorder()

	// when
	sut.handler(context.Background(), rr, req)

	// then
	suite.Equal(http.StatusOK, rr.Code)
	suite.Equal("no-cache", rr.Header().Get("Cache-Control"))
}

func (suite *ServeTestSuite) Test_File_exist_Then_global_headers_are_applied() {

	// given
	cfg := suite.cfg
	cfg.Headers = map[string]string{"X-Test": "test", "Cache-Control": "no-cache"}
	sut := &server{
		cfg:    cfg,
		logger: zerolog.New(os.Stdout),
	}

	req, err := http.NewRequest("GET", "/testfile.json", nil)
	suite.Nil(err)

	rr := httptest.NewRecorder()

	// when
	sut.handler(context.Background(), rr, req)

	// then
	suite.Equal(http.StatusOK, rr.Code)
	suite.Equal("test", rr.Header().Get("X-Test"))
	suite.Equal("no-cache", rr.Header().Get("Cache-Control"))
}

func (suite *ServeTestSuite) Test_File_exist_Then_resource_headers_are_applied() {

	// given
	cfg := suite.cfg
	cfg.Headers = map[string]string{"X-Test": "test", "Cache-Control": "no-cache"}
	cfg.HeadersPerPathRegex = map[string]map[string]string{
		"\\.json": {"X-Test2": "test2", "Cache-Control": "immutable"},
		"\\.txt":  {"X-Test2": "test3"},
	}

	sut := &server{
		cfg:    cfg,
		logger: zerolog.New(os.Stdout),
	}

	req, err := http.NewRequest("GET", "/testfile.json", nil)
	suite.Nil(err)

	rr := httptest.NewRecorder()

	// when
	sut.handler(context.Background(), rr, req)

	// then
	suite.Equal(http.StatusOK, rr.Code)
	suite.Equal("test", rr.Header().Get("X-Test"))
	suite.Equal("test2", rr.Header().Get("X-Test2"))
	suite.Equal("immutable", rr.Header().Get("Cache-Control"))
}
