package fbapi_test

import (
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/daaku/go.fbapi"
	"github.com/daaku/go.flagconfig"
	"github.com/daaku/go.httpcontrol"
)

var (
	defaultHttpTransport = &httpcontrol.Transport{
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
	if err := defaultHttpTransport.Start(); err != nil {
		panic(err)
	}
	defaultFbClient.Transport = defaultHttpTransport
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

	const expected = `GET ` +
		`https://graph.facebook.com/20aa2519-4745-4522-92a9-4522b8edf6e9 got ` +
		`404 Not Found failed with code 803 type OAuthException message (#803) ` +
		`Some of the aliases you requested do not exist: ` +
		`20aa2519-4745-4522-92a9-4522b8edf6e9`

	if err.Error() != expected {
		t.Fatalf(`expected "%s" got "%s"`, expected, err)
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

	const expected = `GET https://graph.facebook.com/ got 400 Bad Request ` +
		`failed with code 100 type GraphMethodException message Unsupported get ` +
		`request`

	if err.Error() != expected {
		t.Fatalf(`expected "%s" got "%s"`, expected, err)
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

	const expected = `GET ` +
		`https://graph.facebook.com/20aa2519-4745-4522-92a9-4522b8edf6e9 got ` +
		`404 Not Found failed with code 803 type OAuthException message (#803) ` +
		`Some of the aliases you requested do not exist: ` +
		`20aa2519-4745-4522-92a9-4522b8edf6e9`

	if err.Error() != expected {
		t.Fatalf(`expected "%s" got "%s"`, expected, err)
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

	const expected = `GET ` +
		`https://graph.facebook.com/20aa2519-4745-4522-92a9-4522b8edf6e9/0 got ` +
		`404 Not Found failed with code 803 type OAuthException message (#803) ` +
		`Some of the aliases you requested do not exist: ` +
		`20aa2519-4745-4522-92a9-4522b8edf6e9`

	if err.Error() != expected {
		t.Fatalf(`expected "%s" got "%s"`, expected, err)
	}

	if res.StatusCode != 404 {
		t.Fatalf("was expecting status 404 but got %d", res.StatusCode)
	}
}
