package fbapi

import (
	"net/url"
	"strconv"
	"strings"
)

// Param augment given url.Values.
type Param interface {
	Set(v url.Values) error
}

// ParamValues builds url.Values from the given Params.
func ParamValues(params ...Param) (v url.Values, err error) {
	v = make(url.Values)
	for _, p := range params {
		err = p.Set(v)
		if err != nil {
			return nil, err
		}
	}
	return v, nil
}

type paramLimit uint64

func (p paramLimit) Set(v url.Values) error {
	v.Add("limit", strconv.FormatUint(uint64(p), 10))
	return nil
}

// ParamLimit specifies a limit. Note, 0 values are also sent.
func ParamLimit(limit uint64) Param {
	return paramLimit(limit)
}

type paramOffset uint64

func (p paramOffset) Set(v url.Values) error {
	if p != 0 {
		v.Add("offset", strconv.FormatUint(uint64(p), 10))
	}
	return nil
}

// ParamOffset specifies the number of items to offset. Note, 0 values are not
// sent.
func ParamOffset(offset uint64) Param {
	return paramOffset(offset)
}

type paramFields []string

func (p paramFields) Set(values url.Values) error {
	if len(p) > 0 {
		values.Set("fields", strings.Join(p, ","))
	}
	return nil
}

// ParamFields specifies the fields to include.
func ParamFields(fields ...string) Param {
	return paramFields(fields)
}

type paramAccessToken string

func (p paramAccessToken) Set(values url.Values) error {
	if p != "" {
		values.Set("access_token", string(p))
	}
	return nil
}

// ParamAccessToken specifies the access_token parameter.
func ParamAccessToken(token string) Param {
	return paramAccessToken(token)
}

type paramDateFormat string

func (p paramDateFormat) Set(values url.Values) error {
	if p != "" {
		values.Add("date_format", string(p))
	}
	return nil
}

// ParamDateFormat specifies the date_format parameter.
func ParamDateFormat(format string) Param {
	return paramDateFormat(format)
}

// Sets the RFC 3339 format that Go expects when unmarshalling time.Time JSON
// values.
var DateFormat = ParamDateFormat(`Y-m-d\TH:i:s\Z`)
