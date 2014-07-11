// Package fbapi provides a client for the Facebook API.
package fbapi

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/facebookgo/httperr"
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

	request  *http.Request
	response *http.Response
	redactor httperr.Redactor
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
		e.redactor,
		e.request,
		e.response,
	).Error()
}

// Wrapper for "error" returned from Facebook APIs.
type errorResponse struct {
	Error Error `json:"error"`
}

// Client for the Facebook API.
type Client struct {
	// The underlying http.RoundTripper to perform the individual requests. When
	// nil http.DefaultTransport will be used.
	Transport http.RoundTripper `inject:""`

	// The base URL to parse relative URLs off. If you pass absolute URLs to Client
	// functions they are used as-is. When nil DefaultBaseURL will be used.
	BaseURL *url.URL

	// Redact sensitive information from errors when true.
	Redact bool
}

// ClientFlag returns a Facebook API Client configured using flags.
func ClientFlag(name string) *Client {
	c := &Client{}
	flag.BoolVar(
		&c.Redact,
		name+".redact",
		true,
		name+" redact known sensitive information from errors",
	)
	return c
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

	res, err := c.transport().RoundTrip(req)
	if err != nil {
		return nil, httperr.RedactError(err, c.Redactor())
	}

	if err := UnmarshalResponse(res, c.Redactor(), result); err != nil {
		return res, err
	}
	return res, nil
}

// Redactor provides a httperr.Redactor to strip the fbapi related sensitive
// information from error messages, for example the access_token.
func (c *Client) Redactor() httperr.Redactor {
	if !c.Redact {
		return httperr.RedactNoOp()
	}
	return redactor
}

// UnmarshalResponse will unmarshal a http.Response from a Facebook API request
// into result, possibly returning an error if the process fails or if the API
// returned an error.
func UnmarshalResponse(res *http.Response, redactor httperr.Redactor, result interface{}) error {
	defer res.Body.Close()

	if res.StatusCode > 399 || res.StatusCode < 200 {
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return httperr.NewError(err, redactor, res.Request, res)
		}

		apiErrorResponse := &errorResponse{
			Error: Error{
				request:  res.Request,
				response: res,
				redactor: redactor,
			},
		}
		err = json.Unmarshal(body, apiErrorResponse)
		if err != nil {
			return httperr.NewError(err, redactor, res.Request, res)
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
		return httperr.NewError(err, redactor, res.Request, res)
	}
	return nil
}
