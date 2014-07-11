// Package fbbatch provides a Client with single call semantics which will
// automatically use Facebook Graph Batch implementation under the hood.
//
// This allows for transparently using batching for greater efficiency. You
// should be aware of how the Facebook Graph API resource limits are applicable
// for your use case and configure the client appropriately.
//
// For the official documentation look at:
// https://developers.facebook.com/docs/reference/api/batch/
package fbbatch

import (
	"encoding/json"
	"errors"
	"flag"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/facebookgo/fbapi"
)

var (
	// DefaultBatchTimeout configures the default timeout after which a batch
	// will be fired.
	DefaultBatchTimeout = flag.Duration(
		"fbbatch.batch_timeout",
		time.Millisecond*10,
		"default batch timeout",
	)

	// DefaultMaxBatchSize configures the default maximum batch size.
	DefaultMaxBatchSize = flag.Int(
		"fbbatch.max_batch_size",
		50,
		"default max batch size",
	)

	errNotStarted = errors.New("fbbatch: client not started")
)

// Request in a Batch.
type Request struct {
	Name        string `json:"name,omitempty"`
	Method      string `json:"method,omitempty"`
	RelativeURL string `json:"relative_url"`
	Body        string `json:"body,omitempty"`
}

// Make a Batch Request from an *http.Request.
func newRequest(hr *http.Request) (*Request, error) {
	// we want relative urls, so we copy and remove the absolute bits
	u := *hr.URL
	u.Scheme = ""
	u.Host = ""

	req := &Request{
		Method:      hr.Method,
		RelativeURL: u.String(),
	}

	if hr.Body != nil {
		bd, err := ioutil.ReadAll(hr.Body)
		if err != nil {
			return nil, err
		}
		req.Body = string(bd)
	}

	return req, nil
}

// Header in a Batch Response.
type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Response in a Batch.
type Response struct {
	Code   int      `json:"code"`
	Header []Header `json:"headers"`
	Body   string   `json:"body,omitempty"`
}

// Convert the Batch Response to a *http.Response or possibly an error.
func (r *Response) httpResponse() (*http.Response, error) {
	header := make(http.Header)
	for _, h := range r.Header {
		header.Add(h.Name, h.Value)
	}

	res := &http.Response{
		Status:        http.StatusText(r.Code),
		StatusCode:    r.Code,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        header,
		Body:          ioutil.NopCloser(strings.NewReader(r.Body)),
		ContentLength: int64(len(r.Body)),
	}
	return res, nil
}

// Batch of Requests.
type Batch struct {
	AccessToken string
	AppID       uint64
	Request     []*Request
}

// BatchDo performs a Batch call. Errors are only returned if the batch itself
// fails, not for the individual requests.
func BatchDo(c *fbapi.Client, b *Batch) ([]*Response, error) {
	v := make(url.Values)

	if b.AccessToken != "" {
		v.Add("access_token", b.AccessToken)
	}
	if b.AppID != 0 {
		v.Add("batch_app_id", strconv.FormatUint(b.AppID, 10))
	}

	j, err := json.Marshal(b.Request)
	if err != nil {
		return nil, err
	}
	v.Add("batch", string(j))

	req, err := http.NewRequest("POST", "/", strings.NewReader(v.Encode()))
	if err != nil {
		return nil, err
	}

	responses := make([]*Response, len(b.Request))
	_, err = c.Do(req, &responses)
	if err != nil {
		return nil, err
	}
	return responses, nil
}

type workResponse struct {
	Response *Response
	Error    error
}

type workRequest struct {
	Request  *Request
	Response chan *workResponse
}

// Client with the same interface as fbapi.Client but one where the underlying
// requests are automatically batched together.
type Client struct {
	Client       *fbapi.Client
	AccessToken  string
	AppID        uint64
	MaxBatchSize int
	BatchTimeout time.Duration
	work         chan *workRequest
	workGroup    sync.WaitGroup
}

// Start the background worker to aggregate and Batch Requests.
func (c *Client) Start() error {
	if int64(c.BatchTimeout) == 0 {
		c.BatchTimeout = *DefaultBatchTimeout
	}
	if c.MaxBatchSize == 0 {
		c.MaxBatchSize = *DefaultMaxBatchSize
	}

	c.work = make(chan *workRequest)
	go c.worker()
	return nil
}

// Stop and gracefully wait for the background worker to finish processing
// pending requests.
func (c *Client) Stop() error {
	close(c.work)
	c.workGroup.Wait()
	return nil
}

func (c *Client) worker() {
	var batchTimeout <-chan time.Time
	var batch []*workRequest
	for {
		select {
		case <-batchTimeout:
			c.workGroup.Add(1)
			go c.send(batch)
			batch = nil
			batchTimeout = nil
		case rr, open := <-c.work:
			if !open {
				c.workGroup.Add(1)
				c.send(batch)
				return
			}
			batch = append(batch, rr)
			if batchTimeout == nil {
				batchTimeout = time.After(c.BatchTimeout)
			}
			if len(batch) >= c.MaxBatchSize {
				c.workGroup.Add(1)
				go c.send(batch)
				batch = nil
				batchTimeout = nil
			}
		}
	}
}

func (c *Client) send(rrs []*workRequest) {
	defer c.workGroup.Done()
	b := &Batch{
		AccessToken: c.AccessToken,
		AppID:       c.AppID,
		Request:     make([]*Request, len(rrs)),
	}
	for i, rr := range rrs {
		b.Request[i] = rr.Request
	}
	res, err := BatchDo(c.Client, b)
	for i, rr := range rrs {
		if err == nil {
			rr.Response <- &workResponse{Response: res[i]}
		} else {
			rr.Response <- &workResponse{Error: err}
		}
	}
}

// Do performs a Graph API request and unmarshal it's response. If the response
// is an error, it will be returned as an error, else it will be unmarshalled
// into the result.
func (c *Client) Do(req *http.Request, result interface{}) (*http.Response, error) {
	if c.work == nil {
		return nil, errNotStarted
	}

	breq, err := newRequest(req)
	if err != nil {
		return nil, err
	}

	wrc := make(chan *workResponse)
	c.work <- &workRequest{Request: breq, Response: wrc}
	wr := <-wrc
	if wr.Error != nil {
		return nil, wr.Error
	}
	hres, err := wr.Response.httpResponse()
	hres.Request = req

	if err := fbapi.UnmarshalResponse(hres, c.Client.Redactor(), result); err != nil {
		return hres, err
	}
	return hres, nil
}
