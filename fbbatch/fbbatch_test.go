package fbbatch_test

import (
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/facebookgo/ensure"
	"github.com/facebookgo/fbapi"
	"github.com/facebookgo/fbapi/fbbatch"
	"github.com/facebookgo/fbapp"
	"github.com/facebookgo/httpcontrol"
)

var (
	defaultHttpTransport = &httpcontrol.Transport{
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
	Name string `json:"name"`
}

func accessToken() string {
	return fmt.Sprintf("%d|%s", defaultApp.ID(), defaultApp.Secret())
}

func init() {
	flag.Parse()
	defaultFbClient.Transport = defaultHttpTransport

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
			{
				RelativeURL: "/facebook?fields=name",
			},
			{
				RelativeURL: "/microsoft?fields=name",
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
		{
			Code: 200,
			Body: `{"name":"Facebook","id":"20531316728"}`,
		},
		{
			Code: 200,
			Body: `{"name":"Microsoft","id":"20528438720"}`,
		},
	}
	ensure.Subset(t, r, expected)
}

func TestPublicGet(t *testing.T) {
	t.Parallel()
	var u user
	res, err := defaultBatchClient.Do(
		&http.Request{
			Method: "GET",
			URL: &url.URL{
				Path: "20531316728",
			},
		},
		&u,
	)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, res.StatusCode, 200)
	ensure.DeepEqual(t, u.Name, "Facebook")
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
	for i := uint(0); i < c.MaxBatchSize; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var u user
			res, err := c.Do(
				&http.Request{
					Method: "GET",
					URL: &url.URL{
						Path: "20531316728",
					},
				},
				&u,
			)
			ensure.Nil(t, err)
			ensure.DeepEqual(t, res.StatusCode, 200)
			ensure.DeepEqual(t, u.Name, "Facebook")
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
