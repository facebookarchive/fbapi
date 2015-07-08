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
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/facebookgo/fbapi"
	"github.com/facebookgo/muster"
)

const (
	defaultPendingWorkCapacity = 1000
	defaultBatchTimeout        = time.Millisecond * 10
	defaultMaxBatchSize        = 50
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
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

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

type musterBatch struct {
	Client       *Client
	WorkRequests []*workRequest
}

func (m *musterBatch) Add(v interface{}) {
	m.WorkRequests = append(m.WorkRequests, v.(*workRequest))
}

func (m *musterBatch) Fire(notifier muster.Notifier) {
	defer notifier.Done()
	b := &Batch{
		AccessToken: m.Client.AccessToken,
		AppID:       m.Client.AppID,
		Request:     make([]*Request, len(m.WorkRequests)),
	}
	for i, rr := range m.WorkRequests {
		b.Request[i] = rr.Request
	}
	res, err := BatchDo(m.Client.Client, b)
	for i, rr := range m.WorkRequests {
		if err == nil {
			rr.Response <- &workResponse{Response: res[i]}
		} else {
			rr.Response <- &workResponse{Error: err}
		}
	}
}

// Client with the same interface as fbapi.Client but one where the underlying
// requests are automatically batched together.
type Client struct {
	Client      *fbapi.Client
	AccessToken string
	AppID       uint64

	// Capacity of log channel. Defaults to 1000.
	PendingWorkCapacity uint

	// Maximum number of items in a batch. Defaults to 50.
	MaxBatchSize uint

	// Amount of time after which to send a pending batch. Defaults to 10ms.
	BatchTimeout time.Duration

	startOnce sync.Once
	startErr  error
	muster    muster.Client
}

// Start the background worker to aggregate and Batch Requests.
func (c *Client) start() error {
	c.startOnce.Do(func() {
		pendingWorkCapacity := c.PendingWorkCapacity
		if pendingWorkCapacity == 0 {
			pendingWorkCapacity = defaultPendingWorkCapacity
		}
		maxBatchSize := c.MaxBatchSize
		if maxBatchSize == 0 {
			maxBatchSize = defaultMaxBatchSize
		}
		batchTimeout := c.BatchTimeout
		if int64(batchTimeout) == 0 {
			batchTimeout = defaultBatchTimeout
		}

		c.muster.BatchMaker = func() muster.Batch { return &musterBatch{Client: c} }
		c.muster.BatchTimeout = batchTimeout
		c.muster.MaxBatchSize = maxBatchSize
		c.muster.PendingWorkCapacity = pendingWorkCapacity
		c.startErr = c.muster.Start()
	})
	return c.startErr
}

// Stop and gracefully wait for the background worker to finish processing
// pending requests.
func (c *Client) Stop() error {
	if err := c.start(); err != nil {
		return err
	}
	return c.muster.Stop()
}

// Do performs a Graph API request and unmarshal it's response. If the response
// is an error, it will be returned as an error, else it will be unmarshalled
// into the result.
func (c *Client) Do(req *http.Request, result interface{}) (*http.Response, error) {
	if err := c.start(); err != nil {
		return nil, err
	}

	breq, err := newRequest(req)
	if err != nil {
		return nil, err
	}

	wrc := make(chan *workResponse, 1)
	c.muster.Work <- &workRequest{Request: breq, Response: wrc}
	wr := <-wrc
	if wr.Error != nil {
		return nil, wr.Error
	}
	hres, err := wr.Response.httpResponse()
	hres.Request = req

	if err := fbapi.UnmarshalResponse(hres, result); err != nil {
		return hres, err
	}
	return hres, nil
}
