// Package fbapi provides wrappers to access the Facebook API.
package fbapi

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/nshah/go.fburl"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

var (
	insecureSSL = flag.Bool(
		"fbapi.insecure", false, "Skip SSL certificate validation.")
	httpClientCache *http.Client
)

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

// Represents a thing that wants to modify the url.Values.
type Values interface {
	Set(url.Values)
}

// Represents an "access_token" for the Facebook API.
type Token string

const (
	PublicToken = Token("")
)

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

// Set the token if necessary.
func (token Token) Set(values url.Values) {
	if token != PublicToken {
		values.Set("access_token", string(token))
	}
}

// String representation as defined by the error interface.
func (e *Error) Error() string {
	return fmt.Sprintf("API call failed with error body:\n%s", string(e.Body))
}

// Disable SSL cert, useful when debugging or hitting internal self-signed certs
func httpClient() *http.Client {
	if httpClientCache == nil {
		transport := &http.Transport{
			Proxy:           http.ProxyFromEnvironment,
			TLSClientConfig: &tls.Config{InsecureSkipVerify: *insecureSSL},
		}
		httpClientCache = &http.Client{
			Transport: transport,
		}
	}
	return httpClientCache
}

// Make a GET Graph API request and get the raw body byte slice.
func GetRaw(path string, values ...Values) ([]byte, error) {
	final := url.Values{}
	for _, v := range values {
		v.Set(final)
	}
	const phpRFC3339 = `Y-m-d\TH:i:s\Z`
	final.Set("date_format", phpRFC3339)
	u := &fburl.URL{
		Scheme:    "https",
		SubDomain: fburl.DGraph,
		Path:      path,
		Values:    final,
	}
	resp, err := httpClient().Get(u.String())
	if err != nil {
		return nil, fmt.Errorf(
			"Request for URL %s failed with error %s.", u.String(), err)
	}
	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf(
			"Request for URL %s failed because body could not be read "+
				"with error %s.",
			u.String(), err)
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
func Get(result interface{}, path string, values ...Values) error {
	b, err := GetRaw(path, values...)
	if err != nil {
		return err
	}
	err = json.Unmarshal(b, result)
	if err != nil {
		return fmt.Errorf(
			"Request for path %s with response %s failed with "+
				"json.Unmarshal error %s.",
			path, string(b), err)
	}
	return nil
}
