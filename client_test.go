package fbapi_test

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
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

func TestErrorString(t *testing.T) {
	e := fbapi.Error{
		Message: "m",
		Type:    "t",
		Code:    42,
	}
	ensure.DeepEqual(t, e.Error(), `fbapi: error code=42 type="t" message="m"`)
}

func TestCustomBaseURL(t *testing.T) {
	t.Parallel()
	baseURL := &url.URL{
		Scheme: "https",
		Host:   "example.com",
		Path:   "/",
	}
	givenErr := errors.New("")
	c := &fbapi.Client{
		BaseURL: baseURL,
		Transport: fTransport(func(r *http.Request) (*http.Response, error) {
			ensure.DeepEqual(t, r.URL.String(), "https://example.com/foo")
			return nil, givenErr
		}),
	}
	_, err := c.Do(&http.Request{
		Method: "GET",
		URL:    &url.URL{Path: "foo"},
	}, nil)
	ensure.True(t, err == givenErr, err)
}

func TestDefaultBaseURL(t *testing.T) {
	t.Parallel()
	givenErr := errors.New("")
	c := &fbapi.Client{
		Transport: fTransport(func(r *http.Request) (*http.Response, error) {
			ensure.DeepEqual(t, r.URL.String(), "https://graph.facebook.com/foo")
			return nil, givenErr
		}),
	}
	_, err := c.Do(&http.Request{
		Method: "GET",
		URL:    &url.URL{Path: "foo"},
	}, nil)
	ensure.True(t, err == givenErr, err)
}

func TestValidResponse(t *testing.T) {
	t.Parallel()
	given := map[string]string{"answer": "42"}
	c := &fbapi.Client{
		Transport: fTransport(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioutil.NopCloser(jsonpipe.Encode(given)),
			}, nil
		}),
	}
	var actual map[string]string
	_, err := c.Do(&http.Request{Method: "GET"}, &actual)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, actual, given)
}

func TestErrorResponse(t *testing.T) {
	t.Parallel()
	givenErr := &fbapi.Error{
		Message: "message42",
		Type:    "type42",
		Code:    42,
	}
	given := map[string]interface{}{"error": givenErr}
	c := &fbapi.Client{
		Transport: fTransport(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Body:       ioutil.NopCloser(jsonpipe.Encode(given)),
			}, nil
		}),
	}
	var actual map[string]string
	_, err := c.Do(&http.Request{Method: "GET"}, &actual)
	ensure.DeepEqual(t, err, givenErr)
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
			BaseURL: u,
		}
		_, err = c.Do(&http.Request{Method: "GET"}, nil)
		ensure.NotNil(t, err)
		ensure.Err(t, err, regexp.MustCompile("(invalid character|EOF)"))
		server.CloseClientConnections()
		server.Close()
	}
}

func TestHTMLResponse(t *testing.T) {
	t.Parallel()
	c := &fbapi.Client{
		Transport: fTransport(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       ioutil.NopCloser(strings.NewReader("<html>")),
			}, nil
		}),
	}
	_, err := c.Do(&http.Request{Method: "GET"}, nil)
	ensure.Err(t, err, regexp.MustCompile("invalid character"))
}

func TestTransportError(t *testing.T) {
	t.Parallel()
	givenErr := errors.New("")
	c := &fbapi.Client{
		Transport: fTransport(func(*http.Request) (*http.Response, error) {
			return nil, givenErr
		}),
	}
	_, err := c.Do(&http.Request{Method: "GET"}, nil)
	ensure.True(t, err == givenErr)
}
