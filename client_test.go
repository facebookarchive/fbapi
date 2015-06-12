package fbapi_test

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"testing"
	"time"

	"github.com/facebookgo/ensure"
	"github.com/facebookgo/fbapi"
	"github.com/facebookgo/flagconfig"
	"github.com/facebookgo/httpcontrol"
)

var (
	defaultHTTPTransport = &httpcontrol.Transport{
		MaxIdleConnsPerHost:   50,
		DialTimeout:           3 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		RequestTimeout:        time.Minute,
		Stats:                 logRequestHandler,
	}
	defaultFbClient = fbapi.ClientFlag("fbapi-test")

	logRequest = flag.Bool(
		"log-requests",
		false,
		"will trigger verbose logging of requests",
	)
)

func init() {
	flag.Usage = flagconfig.Usage
	flagconfig.Parse()
	defaultFbClient.Transport = defaultHTTPTransport
}

func logRequestHandler(stats *httpcontrol.Stats) {
	if *logRequest {
		fmt.Println(stats.String())
		fmt.Println("Header", stats.Request.Header)
	}
}

func TestPublicGet(t *testing.T) {
	t.Parallel()
	user := struct {
		Username string `json:"username"`
	}{}
	res, err := defaultFbClient.Do(
		&http.Request{
			Method: "GET",
			URL: &url.URL{
				Path: "127031120644257",
			},
		},
		&user,
	)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, res.StatusCode, 200)
	ensure.DeepEqual(t, user.Username, "DoctorWho")
}

func TestInvalidGet(t *testing.T) {
	t.Parallel()
	res, err := defaultFbClient.Do(
		&http.Request{
			Method: "GET",
			URL: &url.URL{
				Path: "20aa2519-4745-4522-92a9-4522b8edf6e9",
			},
		},
		nil,
	)
	ensure.Err(t, err, regexp.MustCompile(`failed with code 803`))
	ensure.DeepEqual(t, res.StatusCode, 404)
}

func TestNilURLWithDefaultBaseURL(t *testing.T) {
	t.Parallel()
	res, err := defaultFbClient.Do(&http.Request{Method: "GET"}, nil)
	ensure.Err(t, err, regexp.MustCompile(`failed with code 100`))
	ensure.DeepEqual(t, res.StatusCode, 400)
}

func TestNilURLWithBaseURL(t *testing.T) {
	t.Parallel()
	client := &fbapi.Client{
		BaseURL: &url.URL{
			Scheme: "https",
			Host:   "graph.facebook.com",
			Path:   "/20aa2519-4745-4522-92a9-4522b8edf6e9",
		},
	}
	res, err := client.Do(&http.Request{Method: "GET"}, nil)
	ensure.Err(t, err, regexp.MustCompile(`failed with code 803`))
	ensure.DeepEqual(t, res.StatusCode, 404)
}

func TestRelativeToBaseURL(t *testing.T) {
	t.Parallel()
	client := &fbapi.Client{
		BaseURL: &url.URL{
			Scheme: "https",
			Host:   "graph.facebook.com",
			Path:   "/20aa2519-4745-4522-92a9-4522b8edf6e9/",
		},
	}
	res, err := client.Do(
		&http.Request{Method: "GET", URL: &url.URL{Path: "0"}},
		nil,
	)
	ensure.Err(t, err, regexp.MustCompile(`failed with code 803`))
	ensure.DeepEqual(t, res.StatusCode, 404)
}

func TestPublicGetDiscardBody(t *testing.T) {
	t.Parallel()
	res, err := defaultFbClient.Do(
		&http.Request{
			Method: "GET",
			URL: &url.URL{
				Path: "5526183",
			},
		},
		nil,
	)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, res.StatusCode, 200)
}

func TestServerAbort(t *testing.T) {
	t.Parallel()
	for _, code := range []int{200, 500} {
		server := httptest.NewServer(
			http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					w.Header().Add("Content-Length", "4000")
					w.WriteHeader(code)
					w.Write(bytes.Repeat([]byte("a"), 3000))
				},
			),
		)

		u, err := url.Parse(server.URL)
		ensure.Nil(t, err)

		c := &fbapi.Client{
			Transport: defaultHTTPTransport,
			BaseURL:   u,
		}
		res := make(map[string]interface{})
		_, err = c.Do(&http.Request{Method: "GET"}, res)
		ensure.NotNil(t, err)
		ensure.StringContains(t, err.Error(), fmt.Sprintf(`GET %s`, server.URL))
		server.CloseClientConnections()
		server.Close()
	}
}

func TestHTMLResponse(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(500)
				w.Write([]byte("<html></html>"))
			},
		),
	)

	u, err := url.Parse(server.URL)
	ensure.Nil(t, err)

	c := &fbapi.Client{
		Transport: defaultHTTPTransport,
		BaseURL:   u,
	}
	res := make(map[string]interface{})
	_, err = c.Do(&http.Request{Method: "GET"}, res)
	ensure.Err(t, err, regexp.MustCompile(`got 500 Internal Server Error`))
	server.CloseClientConnections()
	server.Close()
}

type errorTransport struct{}

func (e errorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, errors.New("42")
}

func TestTransportError(t *testing.T) {
	t.Parallel()
	c := &fbapi.Client{
		Transport: errorTransport{},
	}
	res := make(map[string]interface{})
	_, err := c.Do(&http.Request{Method: "GET"}, res)
	ensure.Err(t, err, regexp.MustCompile(`42`))
}
