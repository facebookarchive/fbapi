// Package fbapi provides wrappers to access the Facebook API.
package fbapi

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/daaku/go.fburl"
)

const redactedStub = "$1=-- XX -- REDACTED -- XX --"

var cleanURLRegExp = regexp.MustCompile("(access_token|client_secret)=([^&]*)")

// Represents a thing that wants to modify the url.Values.
type Values interface {
	Set(url.Values)
}

// Generic Page options for list type queries.
type Page struct {
	Limit  int
	Offset int
}

// Set the corresponding values for the Page.
func (page Page) Set(values url.Values) {
	if page.Limit != 0 {
		values.Set("limit", strconv.Itoa(page.Limit))
	}
	if page.Offset != 0 {
		values.Set("offset", strconv.Itoa(page.Offset))
	}
}

// A slice of field names.
type Fields []string

// For selecting fields.
func (fields Fields) Set(values url.Values) {
	if len(fields) > 0 {
		values.Set("fields", strings.Join(fields, ","))
	}
}

// Represents an "access_token" for the Facebook API.
type Token string

const PublicToken = Token("")

// Set the token if necessary.
func (token Token) Set(values url.Values) {
	if token != PublicToken {
		values.Set("access_token", string(token))
	}
}

// An Error from the API.
type Error struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    int    `json:"code"`
	Body    []byte
}

// Wrapper for "error"
type errorResponse struct {
	Error Error `json:"error"`
}

// String representation as defined by the error interface.
func (e *Error) Error() string {
	return fmt.Sprintf("API call failed with error body:\n%s", string(e.Body))
}

type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Facebook API Client.
type Client struct {
	Redact     bool // When true known sensitive information will be stripped from errors
	HttpClient HttpClient
}

// remove known sensitive tokens from data
func (c *Client) cleanURL(url string) string {
	if c.Redact {
		return cleanURLRegExp.ReplaceAllString(url, redactedStub)
	}
	return url
}

// Make a GET Graph API request and get the raw body byte slice.
func (c *Client) GetRaw(path string, values url.Values) ([]byte, error) {
	const phpRFC3339 = `Y-m-d\TH:i:s\Z`
	values.Set("date_format", phpRFC3339)
	u := &fburl.URL{
		Scheme:    "https",
		SubDomain: fburl.DGraph,
		Path:      path,
		Values:    values,
	}
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf(
			"Request for URL %s failed with error %s.", c.cleanURL(u.String()), err)
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf(
			"Request for URL %s failed because body could not be read "+
				"with error %s.",
			c.cleanURL(u.String()), err)
	}
	if resp.StatusCode > 399 || resp.StatusCode < 200 {
		apiError := &errorResponse{Error{Body: b}}
		err = json.Unmarshal(b, apiError)
		if err != nil {
			return nil, fmt.Errorf(
				"Parsing error response failed with %s:\n%s", err, string(b))
		}
		return nil, &apiError.Error
	}
	return b, nil
}

// Make a GET Graph API request.
func (c *Client) Get(result interface{}, path string, values ...Values) error {
	final := url.Values{}
	for _, v := range values {
		v.Set(final)
	}
	b, err := c.GetRaw(path, final)
	if err != nil {
		return err
	}
	err = json.Unmarshal(b, result)
	if err != nil {
		return fmt.Errorf(
			"Request for path %s with response %s failed with "+
				"json.Unmarshal error %s.",
			c.cleanURL(path), string(b), err)
	}
	return nil
}

// A Flag configured Client.
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
