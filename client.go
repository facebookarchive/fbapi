// Package fbapi provides a client for the Facebook API.
package fbapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
)

var defaultBaseURL = &url.URL{
	Scheme: "https",
	Host:   "graph.facebook.com",
	Path:   "/",
}

// An Error from the API.
type Error struct {
	// These are provided by the Facebook API and may not always be available.
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    int    `json:"code"`
}

func (e *Error) Error() string {
	var b bytes.Buffer
	fmt.Fprintf(&b, "fbapi: error")
	if e.Code != 0 {
		fmt.Fprintf(&b, " code=%d", e.Code)
	}
	if e.Type != "" {
		fmt.Fprintf(&b, " type=%q", e.Type)
	}
	if e.Message != "" {
		fmt.Fprintf(&b, " message=%q", e.Message)
	}
	return b.String()
}

// Client for the Facebook API.
type Client struct {
	// The underlying http.RoundTripper to perform the individual requests. When
	// nil http.DefaultTransport will be used.
	Transport http.RoundTripper `inject:""`

	// The base URL to parse relative URLs off. If you pass absolute URLs to Client
	// functions they are used as-is. When nil https://graph.facebook.com/ will
	// be used.
	BaseURL *url.URL
}

func (c *Client) transport() http.RoundTripper {
	if c.Transport == nil {
		return http.DefaultTransport
	}
	return c.Transport
}

// Do performs a Graph API request and unmarshal it's response. If the response
// is an error, it will be returned as an error, else it will be unmarshalled
// into the result.
func (c *Client) Do(req *http.Request, result interface{}) (*http.Response, error) {
	req.Proto = "HTTP/1.1"
	req.ProtoMajor = 1
	req.ProtoMinor = 1

	if req.URL == nil {
		if c.BaseURL == nil {
			req.URL = defaultBaseURL
		} else {
			req.URL = c.BaseURL
		}
	} else {
		if !req.URL.IsAbs() {
			if c.BaseURL == nil {
				req.URL = defaultBaseURL.ResolveReference(req.URL)
			} else {
				req.URL = c.BaseURL.ResolveReference(req.URL)
			}
		}
	}

	if req.Host == "" {
		req.Host = req.URL.Host
	}

	if req.Header == nil {
		req.Header = make(http.Header)
	}

	res, err := c.transport().RoundTrip(req)
	if err != nil {
		return nil, err
	}

	if err := UnmarshalResponse(res, result); err != nil {
		return res, err
	}
	return res, nil
}

// UnmarshalResponse will unmarshal a http.Response from a Facebook API request
// into result, possibly returning an error if the process fails or if the API
// returned an error.
func UnmarshalResponse(res *http.Response, result interface{}) error {
	defer res.Body.Close()

	if res.StatusCode > 399 || res.StatusCode < 200 {
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return err
		}

		var apiErrorResponse struct {
			Error Error `json:"error"`
		}
		if err := json.Unmarshal(body, &apiErrorResponse); err != nil {
			return err
		}
		return &apiErrorResponse.Error
	}

	var err error
	if result == nil {
		_, err = io.Copy(ioutil.Discard, res.Body)
	} else {
		err = json.NewDecoder(res.Body).Decode(result)
	}
	if err != nil {
		return err
	}
	return nil
}
