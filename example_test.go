package fbapi_test

import (
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/daaku/go.fbapi"
)

func Example() {
	client := &fbapi.Client{}
	var user struct {
		ID   uint64 `json:"id,string"`
		Name string `json:"name"`
	}
	_, err := client.Do(
		&http.Request{Method: "GET", URL: &url.URL{Path: "naitik"}},
		&user,
	)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Printf("%+v", user)

	// Output: {ID:5526183 Name:Naitik Shah}
}
