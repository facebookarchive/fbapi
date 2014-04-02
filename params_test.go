package fbapi_test

import (
	"errors"
	"net/url"
	"reflect"
	"testing"

	"github.com/facebookgo/fbapi"
)

const paramWithErrorMessage = "foo"

type paramWithError struct{}

func (p paramWithError) Set(v url.Values) error {
	return errors.New(paramWithErrorMessage)
}

func TestParams(t *testing.T) {
	cases := []struct {
		Params   []fbapi.Param
		Expected url.Values
	}{
		{
			Params:   []fbapi.Param{fbapi.ParamLimit(42)},
			Expected: url.Values{"limit": []string{"42"}},
		},
		{
			Params:   []fbapi.Param{fbapi.ParamOffset(42)},
			Expected: url.Values{"offset": []string{"42"}},
		},
		{
			Params:   []fbapi.Param{fbapi.ParamFields("abc", "def")},
			Expected: url.Values{"fields": []string{"abc,def"}},
		},
		{
			Params:   []fbapi.Param{fbapi.ParamAccessToken("42")},
			Expected: url.Values{"access_token": []string{"42"}},
		},
		{
			Params:   []fbapi.Param{fbapi.ParamDateFormat("42")},
			Expected: url.Values{"date_format": []string{"42"}},
		},
	}

	for _, c := range cases {
		v, err := fbapi.ParamValues(c.Params...)
		if err != nil {
			t.Errorf("case %+v got error %s", c, err)
		}
		if !reflect.DeepEqual(c.Expected, v) {
			t.Fatalf("case\n%+v\nactual:\n%+v", c, v)
		}
	}
}

func TestParamsError(t *testing.T) {
	_, err := fbapi.ParamValues(paramWithError{})
	if err == nil {
		t.Fatal("was expecting error")
	}
	if err.Error() != paramWithErrorMessage {
		t.Fatalf("expected %s got %s", paramWithErrorMessage, err)
	}
}
