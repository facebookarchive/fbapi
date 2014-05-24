package fbbatch_test

import (
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/facebookgo/fbapi"
	"github.com/facebookgo/fbapi/fbbatch"
)

func Example() {
	client := &fbbatch.Client{
		Client: &fbapi.Client{},
		AppID:  161808054014511,
	}
	if err := client.Start(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer client.Stop()

	var show struct {
		ID      uint64 `json:"id,string"`
		Name    string `json:"name"`
		Network string `json:"network"`
	}
	_, err := client.Do(
		&http.Request{Method: "GET", URL: &url.URL{Path: "DoctorWho"}},
		&show,
	)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
	fmt.Printf("%+v", show)

	// Output: {ID:127031120644257 Name:Doctor Who Network:BBC}
}
