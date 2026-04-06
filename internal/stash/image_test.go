package stash

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestFetchImageAttachesAPIKeyForStashHost(t *testing.T) {
	var gotAPIKey string

	client := NewClient("http://stash.local", "secret-key", false)
	client.http = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotAPIKey = r.Header.Get("ApiKey")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"image/jpeg"}},
			Body:       io.NopCloser(strings.NewReader("jpeg-bytes")),
		}, nil
	})}

	image, err := client.FetchImage(context.Background(), "/image/1.jpg")
	if err != nil {
		t.Fatalf("FetchImage: %v", err)
	}
	if got, want := gotAPIKey, "secret-key"; got != want {
		t.Fatalf("api key mismatch: got %q want %q", got, want)
	}
	if got, want := string(image.Data), "jpeg-bytes"; got != want {
		t.Fatalf("image bytes mismatch: got %q want %q", got, want)
	}
}
