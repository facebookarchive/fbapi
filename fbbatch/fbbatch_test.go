package fbbatch

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/facebookgo/ensure"
	"github.com/facebookgo/fbapi"
	"github.com/facebookgo/jsonpipe"
)

type fTransport func(*http.Request) (*http.Response, error)

func (f fTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type fReader func([]byte) (int, error)

func (f fReader) Read(p []byte) (int, error) { return f(p) }

func TestNewRequest(t *testing.T) {
	const (
		method = "GET"
		path   = "/me"
		body   = "body42"
	)
	hr := &http.Request{
		Method: method,
		URL: &url.URL{
			Scheme: "https",
			Host:   "graph.facebook.com",
			Path:   path,
		},
		Body: ioutil.NopCloser(strings.NewReader(body)),
	}
	br, err := newRequest(hr)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, br, &Request{
		Method:      method,
		RelativeURL: path,
		Body:        body,
	})
}

func TestNewRequestBodyReadError(t *testing.T) {
	givenErr := errors.New("")
	_, err := newRequest(&http.Request{
		URL: &url.URL{},
		Body: ioutil.NopCloser(fReader(func([]byte) (int, error) {
			return 0, givenErr
		})),
	})
	ensure.True(t, err == givenErr, err)
}

func TestHTTPResponse(t *testing.T) {
	const (
		code       = http.StatusOK
		body       = "body42"
		headerKey  = "Foo"
		headerVal1 = "bar"
		headerVal2 = "baz"
	)
	br := Response{
		Code: code,
		Header: []Header{
			{Name: headerKey, Value: headerVal1},
			{Name: headerKey, Value: headerVal2},
		},
		Body: body,
	}
	hr, err := br.httpResponse()
	ensure.Nil(t, err)
	ensure.DeepEqual(t, hr, &http.Response{
		Status:        http.StatusText(code),
		StatusCode:    code,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        http.Header{headerKey: []string{headerVal1, headerVal2}},
		Body:          ioutil.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
	})
}

func TestBatchDo(t *testing.T) {
	const (
		method      = "GET"
		relativeURL = "/me"
		accessToken = "at"
		appID       = 42
	)
	given := []*Response{{Code: 42}}
	br := &Request{
		Method:      method,
		RelativeURL: relativeURL,
	}
	b := &Batch{
		AccessToken: accessToken,
		AppID:       appID,
		Request:     []*Request{br},
	}
	c := &fbapi.Client{
		Transport: fTransport(func(r *http.Request) (*http.Response, error) {
			ensure.Nil(t, r.ParseForm())
			ensure.DeepEqual(t, r.URL.String(), "https://graph.facebook.com/")
			ensure.DeepEqual(t, r.Method, "POST")
			ensure.DeepEqual(t, r.PostFormValue("access_token"), accessToken)
			ensure.DeepEqual(t, r.PostFormValue("batch_app_id"), fmt.Sprint(appID))
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioutil.NopCloser(jsonpipe.Encode(given)),
			}, nil
		}),
	}
	actual, err := BatchDo(c, b)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, actual, given)
}

func TestBatchDoTransportError(t *testing.T) {
	givenErr := errors.New("")
	c := &fbapi.Client{
		Transport: fTransport(func(*http.Request) (*http.Response, error) {
			return nil, givenErr
		}),
	}
	_, err := BatchDo(c, &Batch{})
	ensure.True(t, err == givenErr, err)
}

func TestClientDo(t *testing.T) {
	given := map[string]string{"answer": "42"}
	givenJSON, err := json.Marshal(given)
	ensure.Nil(t, err)
	wrapped := []map[string]interface{}{
		{
			"code": http.StatusOK,
			"body": string(givenJSON),
		},
	}
	c := &Client{
		Client: &fbapi.Client{
			Transport: fTransport(func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(jsonpipe.Encode(wrapped)),
				}, nil
			}),
		},
	}
	var actual map[string]string
	_, err = c.Do(&http.Request{
		Method: "GET",
		URL:    &url.URL{},
	}, &actual)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, actual, given)
}

func TestStopClient(t *testing.T) {
	ensure.Nil(t, (&Client{Client: &fbapi.Client{}}).Stop())
}
