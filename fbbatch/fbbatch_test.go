package fbbatch_test

import (
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/facebookgo/ensure"
	"github.com/facebookgo/fbapi"
	"github.com/facebookgo/fbapi/fbbatch"
)

var (
	defaultFbClient    = &fbapi.Client{Redact: true}
	defaultBatchClient = &fbbatch.Client{
		Client:      defaultFbClient,
		AccessToken: "161808054014511|e82319ae149d25b14217f9a34064b173",
	}
)

type user struct {
	Name string `json:"name"`
}

func init() {
	if err := defaultBatchClient.Start(); err != nil {
		panic(err)
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
		AccessToken: defaultBatchClient.AccessToken,
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
		AccessToken:  defaultBatchClient.AccessToken,
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
		AccessToken:  defaultBatchClient.AccessToken,
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
