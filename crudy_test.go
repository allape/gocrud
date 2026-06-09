package gocrud

import (
	"errors"
	"net/http"
	"testing"
)

func TestNewCrudy(t *testing.T) {
	_, err := NewCrudy[User]("I am an invalid URL, ^^^%%%$$$$")
	if err == nil {
		t.Fatal("expected an invalid url error")
		return
	}

	_, err = NewCrudy[User]("")
	if !errors.Is(err, ErrorBaseURLRequired) {
		t.Fatalf("expected an ErrorBaseURLRequired error, but got %v", err)
	}

	userAddr := "http://127.0.0.1:8080/user"

	crudy, err := NewCrudy[User](userAddr)
	if err != nil {
		t.Fatal(err)
	}

	if crudy.baseURL != userAddr {
		t.Fatalf("got %s, want %s", crudy.baseURL, userAddr)
	}
	if crudy.httpClient != http.DefaultClient {
		t.Fatalf("got %v, want %v", crudy.httpClient, http.DefaultClient)
	}
	if crudy.okayHttpStatusRange == nil {
		t.Fatalf("got nil")
	}
	if crudy.okayHttpStatusRange[0] != http.StatusOK {
		t.Fatalf("got %v, want %v", crudy.okayHttpStatusRange[0], http.StatusOK)
	}
	if crudy.okayHttpStatusRange[1] != http.StatusMultipleChoices {
		t.Fatalf("got %v, want %v", crudy.okayHttpStatusRange[1], http.StatusMultipleChoices)
	}

	newHttpClient := &http.Client{}

	crudy, err = NewCrudy[User](
		userAddr,
		CrudyPageOptions[User]{
			DefaultSize: 135,
		},
		CrudyBasicOptions[User]{
			HttpClient:          newHttpClient,
			OkayHttpStatusRange: &HttpStatusRange{123, 456},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if crudy.baseURL != userAddr {
		t.Fatalf("got %s, want %s", crudy.baseURL, userAddr)
	}
	if crudy.defaultPageSize != 135 {
		t.Fatalf("got %d, want 135", crudy.defaultPageSize)
	}
	if crudy.httpClient != newHttpClient {
		t.Fatalf("got %v, want %v", crudy.httpClient, newHttpClient)
	}
	if crudy.okayHttpStatusRange == nil {
		t.Fatalf("got nil")
	}
	if crudy.okayHttpStatusRange[0] != 123 {
		t.Fatalf("got %v, want %v", crudy.okayHttpStatusRange[0], 123)
	}
	if crudy.okayHttpStatusRange[1] != 456 {
		t.Fatalf("got %v, want %v", crudy.okayHttpStatusRange[1], 456)
	}
}
