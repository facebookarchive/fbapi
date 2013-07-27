// Package fbapi provides a client for the Facebook API.
package fbapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/daaku/go.httperr"
)

var redactor = httperr.RedactRegexp(
	regexp.MustCompile("(access_token|client_secret)=([^&]*)"),
	"$1=-- XX -- REDACTED -- XX --",
)

// The default base URL for the API.
var DefaultBaseURL = &url.URL{
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

	request  *http.Request  `json:"-"`
	response *http.Response `json:"-"`
	client   *Client
}

func (e *Error) Error() string {
	var parts []string
	if e.Code != 0 {
		parts = append(parts, fmt.Sprintf("code %d", e.Code))
	}
	if e.Type != "" {
		parts = append(parts, fmt.Sprintf("type %s", e.Type))
	}
	if e.Message != "" {
		parts = append(parts, fmt.Sprintf("message %s", e.Message))
	}
	return httperr.NewError(
		errors.New(strings.Join(parts, " ")),
		e.client.redactor(),
		e.request,
		e.response,
	).Error()
}

// Wrapper for "error" returned from Facebook APIs.
type errorResponse struct {
	Error Error `json:"error"`
}

// Facebook API Client.
type Client struct {
	Transport http.RoundTripper
	BaseURL   *url.URL
	Redact    bool // Redact sensitive information from errors when true
}

// Perform a Graph API request and unmarshal it's response. If the response is
// an error, it will be returned as an error, else it will be unmarshalled into
// the result.
func (c *Client) Do(req *http.Request, result interface{}) (*http.Response, error) {
	req.Proto = "HTTP/1.1"
	req.ProtoMajor = 1
	req.ProtoMinor = 1

	if req.URL == nil {
		if c.BaseURL == nil {
			req.URL = DefaultBaseURL
		} else {
			req.URL = c.BaseURL
		}
	} else {
		if !req.URL.IsAbs() {
			if c.BaseURL == nil {
				req.URL = DefaultBaseURL.ResolveReference(req.URL)
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

	res, err := c.Transport.RoundTrip(req)
	if err != nil {
		return nil, httperr.RedactError(err, c.redactor())
	}
	defer res.Body.Close()

	if res.StatusCode > 399 || res.StatusCode < 200 {
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return res, httperr.NewError(err, c.redactor(), req, res)
		}

		apiErrorResponse := &errorResponse{
			Error: Error{
				request:  req,
				response: res,
				client:   c,
			},
		}
		err = json.Unmarshal(body, apiErrorResponse)
		if err != nil {
			return res, httperr.NewError(err, c.redactor(), req, res)
		}
		return res, &apiErrorResponse.Error
	}

	if result == nil {
		_, err = io.Copy(ioutil.Discard, res.Body)
	} else {
		err = json.NewDecoder(res.Body).Decode(result)
	}
	if err != nil {
		return res, httperr.NewError(err, c.redactor(), req, res)
	}
	return res, nil
}

func (c *Client) redactor() httperr.Redactor {
	if !c.Redact {
		return httperr.RedactNoOp()
	}
	return redactor
}
