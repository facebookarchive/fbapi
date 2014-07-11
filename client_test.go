package fbapi_test

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

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
				Path: "5526183",
			},
		},
		&user,
	)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 {
		t.Fatalf("was expecting status 200 but got %d", res.StatusCode)
	}
	if user.Username != "naitik" {
		t.Fatalf("was expecting naitik but got %s", user.Username)
	}
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
	if err == nil {
		t.Fatal("was expecting error")
	}

	const expectedPrefix = `GET ` +
		`https://graph.facebook.com/20aa2519-4745-4522-92a9-4522b8edf6e9 got `
	const expectedSuffix = `failed with code 803 type OAuthException message (#803) ` +
		`Some of the aliases you requested do not exist: ` +
		`20aa2519-4745-4522-92a9-4522b8edf6e9`

	if !strings.HasPrefix(err.Error(), expectedPrefix) ||
		!strings.HasSuffix(err.Error(), expectedSuffix) {
		t.Fatalf(`expected "%s.*%s" got "%s"`, expectedPrefix, expectedSuffix, err)
	}

	if res.StatusCode != 404 {
		t.Fatalf("was expecting status 404 but got %d", res.StatusCode)
	}
}

func TestNilURLWithDefaultBaseURL(t *testing.T) {
	t.Parallel()
	res, err := defaultFbClient.Do(&http.Request{Method: "GET"}, nil)
	if err == nil {
		t.Fatal("was expecting error")
	}

	const expectedPrefix = `GET https://graph.facebook.com/ got`
	const expectedSuffix = `failed with code 100 type GraphMethodException message Unsupported get ` +
		`request.`

	if !strings.HasPrefix(err.Error(), expectedPrefix) ||
		!strings.HasSuffix(err.Error(), expectedSuffix) {
		t.Fatalf(`expected "%s.*%s" got "%s"`, expectedPrefix, expectedSuffix, err)
	}

	if res.StatusCode != 400 {
		t.Fatalf("was expecting status 400 but got %d", res.StatusCode)
	}
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
	if err == nil {
		t.Fatal("was expecting error")
	}

	const expectedPrefix = `GET ` +
		`https://graph.facebook.com/20aa2519-4745-4522-92a9-4522b8edf6e9`
	const expectedSuffix = `failed with code 803 type OAuthException message (#803) ` +
		`Some of the aliases you requested do not exist: ` +
		`20aa2519-4745-4522-92a9-4522b8edf6e9`

	if !strings.HasPrefix(err.Error(), expectedPrefix) ||
		!strings.HasSuffix(err.Error(), expectedSuffix) {
		t.Fatalf(`expected "%s.*%s" got "%s"`, expectedPrefix, expectedSuffix, err)
	}

	if res.StatusCode != 404 {
		t.Fatalf("was expecting status 404 but got %d", res.StatusCode)
	}
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
	if err == nil {
		t.Fatal("was expecting error")
	}

	const expectedPrefix = `GET ` +
		`https://graph.facebook.com/20aa2519-4745-4522-92a9-4522b8edf6e9/0`
	const expectedSuffix = `failed with code 803 type OAuthException message (#803) ` +
		`Some of the aliases you requested do not exist: ` +
		`20aa2519-4745-4522-92a9-4522b8edf6e9`

	if !strings.HasPrefix(err.Error(), expectedPrefix) ||
		!strings.HasSuffix(err.Error(), expectedSuffix) {
		t.Fatalf(`expected "%s.*%s" got "%s"`, expectedPrefix, expectedSuffix, err)
	}

	if res.StatusCode != 404 {
		t.Fatalf("was expecting status 404 but got %d", res.StatusCode)
	}
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
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 {
		t.Fatalf("was expecting status 200 but got %d", res.StatusCode)
	}
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
		if err != nil {
			t.Fatal(err)
		}

		c := &fbapi.Client{
			Transport: defaultHTTPTransport,
			BaseURL:   u,
		}
		res := make(map[string]interface{})
		_, err = c.Do(&http.Request{Method: "GET"}, res)
		if err == nil {
			t.Fatalf("was expecting an error instead got %v", res)
		}
		expected := fmt.Sprintf(`GET %s`, server.URL)
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf(
				`did not contain expected error "%s" instead got "%s"`,
				expected,
				err,
			)
		}
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
	if err != nil {
		t.Fatal(err)
	}

	c := &fbapi.Client{
		Transport: defaultHTTPTransport,
		BaseURL:   u,
	}
	res := make(map[string]interface{})
	_, err = c.Do(&http.Request{Method: "GET"}, res)
	if err == nil {
		t.Fatalf("was expecting an error instead got %v", res)
	}
	expected := fmt.Sprintf(
		`GET %s got 500 Internal Server Error failed with invalid character '<' `+
			`looking for beginning of value`,
		server.URL,
	)
	if err.Error() != expected {
		t.Fatalf(
			`did not contain expected error "%s" instead got "%s"`,
			expected,
			err,
		)
	}
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
	if err == nil {
		t.Fatalf("was expecting an error instead got %v", res)
	}
	const expected = "42"
	if err.Error() != expected {
		t.Fatalf(
			`did not contain expected error "%s" instead got "%s"`,
			expected,
			err,
		)
	}
}
