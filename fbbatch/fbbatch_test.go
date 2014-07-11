package fbbatch_test

import (
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/facebookgo/fbapi"
	"github.com/facebookgo/fbapi/fbbatch"
	"github.com/facebookgo/fbapp"
	"github.com/facebookgo/httpcontrol"
	"github.com/facebookgo/subset"
)

var (
	defaultHTTPTransport = &httpcontrol.Transport{
		MaxIdleConnsPerHost:   50,
		DialTimeout:           3 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		RequestTimeout:        time.Minute,
		Stats:                 logRequestHandler,
	}
	defaultFbClient    = fbapi.ClientFlag("fbapi-test")
	defaultApp         = fbapp.Flag("fbapp-test")
	defaultBatchClient = &fbbatch.Client{
		Client: defaultFbClient,
	}

	logRequest = flag.Bool(
		"log-requests",
		false,
		"will trigger verbose logging of requests",
	)
)

type user struct {
	Username string `json:"username"`
}

func accessToken() string {
	return fmt.Sprintf("%d|%s", defaultApp.ID(), defaultApp.Secret())
}

func init() {
	flag.Parse()
	defaultFbClient.Transport = defaultHTTPTransport

	// default app for testing
	if defaultApp.ID() == 0 {
		defaultApp = fbapp.New(
			161808054014511,
			"e82319ae149d25b14217f9a34064b173",
			"gofbapi",
		)
	}

	if err := defaultBatchClient.Start(); err != nil {
		panic(err)
	}
	defaultBatchClient.AccessToken = accessToken()
}

func logRequestHandler(stats *httpcontrol.Stats) {
	if *logRequest {
		fmt.Println(stats.String())
		fmt.Println("Header", stats.Request.Header)
	}
}

func TestNotStarted(t *testing.T) {
	t.Parallel()
	c := &fbbatch.Client{}

	_, err := c.Do(
		&http.Request{
			Method: "GET",
			URL: &url.URL{
				Path: "5526183",
			},
		},
		nil,
	)
	if err == nil {
		t.Fatal("was expecting error")
	}
	if err.Error() != "fbbatch: client not started" {
		t.Fatalf("did not get expected error, got %s", err)
	}
}

func TestBatchGets(t *testing.T) {
	b := &fbbatch.Batch{
		AccessToken: accessToken(),
		Request: []*fbbatch.Request{
			&fbbatch.Request{
				RelativeURL: "/naitik?fields=first_name",
			},
			&fbbatch.Request{
				RelativeURL: "/shwetanshah?fields=first_name",
			},
		},
	}

	r, err := fbbatch.BatchDo(defaultFbClient, b)
	if err != nil {
		t.Fatal(err)
	}

	if len(r) != 2 {
		t.Fatal("was expecting 2 results")
	}

	expected := []*fbbatch.Response{
		&fbbatch.Response{
			Code: 200,
			Body: `{"first_name":"Naitik","id":"5526183"}`,
		},
		&fbbatch.Response{
			Code: 200,
			Body: `{"first_name":"Shweta","id":"61302481"}`,
		},
	}
	if !subset.Check(expected, r) {
		t.Fatal("did not find expected subset")
	}
}

func TestPublicGet(t *testing.T) {
	t.Parallel()
	var u user
	res, err := defaultBatchClient.Do(
		&http.Request{
			Method: "GET",
			URL: &url.URL{
				Path: "5526183",
			},
		},
		&u,
	)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 {
		t.Fatalf("was expecting status 200 but got %d", res.StatusCode)
	}
	if u.Username != "naitik" {
		t.Fatalf("was expecting naitik but got %s", u.Username)
	}
}

func TestMaxBatchSize(t *testing.T) {
	t.Parallel()
	c := &fbbatch.Client{
		AccessToken:  accessToken(),
		Client:       defaultFbClient,
		BatchTimeout: time.Second,
		MaxBatchSize: 2,
	}
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < c.MaxBatchSize; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var u user
			res, err := c.Do(
				&http.Request{
					Method: "GET",
					URL: &url.URL{
						Path: "5526183",
					},
				},
				&u,
			)
			if err != nil {
				t.Fatal(err)
			}
			if res.StatusCode != 200 {
				t.Fatalf("was expecting status 200 but got %d", res.StatusCode)
			}
			if u.Username != "naitik" {
				t.Fatalf("was expecting naitik but got %s", u.Username)
			}
		}()
	}
	wg.Wait()
}

func TestStopClient(t *testing.T) {
	t.Parallel()
	c := &fbbatch.Client{
		AccessToken:  accessToken(),
		Client:       defaultFbClient,
		BatchTimeout: time.Second,
	}
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	if err := c.Stop(); err != nil {
		t.Fatal(err)
	}
}
