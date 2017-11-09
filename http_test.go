package http_test

import (
	stdhttp "net/http"
	"testing"

	"bytes"
	"fmt"
	"github.com/AlkBur/http"
)

func TestHttp(t *testing.T) {
	const (
		port    = 9090
		reqBody = `{"abc":123}`
		resBody = `{"xyz":456}`
	)

	srv := http.New(fmt.Sprintf(":%d", port))
	srv.Handler = Index
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			t.Fatal("unable to serve:", err)
		}
	}()

	resp, err := stdhttp.Post(fmt.Sprintf("http://localhost:%d", port), "application/json", bytes.NewReader([]byte(reqBody)))
	if err != nil {
		t.Fatal("post failed:", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected status code %s, got: %s", 200, resp.StatusCode)
	}
	//if resp.Body.Read() != reqBody {
	//	t.Fatalf("expected body '%s', got: '%s'", reqBody, string(th.request.body))
	//}
}

func Index(res *http.Response, req *http.Request) {

}
