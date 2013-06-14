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
	defaultHttpClient = &http.Client{Transport: defaultHttpTransport}
	defaultFbClient   = &fbapi.Client{
		HttpClient: defaultHttpClient,
	}

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
