package gocrud

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func mustBeURL(urlStr string, searchParams ...SearchParams) *url.URL {
	u, err := url.Parse(urlStr)
	if err != nil {
		panic(err)
	}

	query := u.Query()
	for _, params := range searchParams {
		for key, value := range params {
			query.Add(key, value)
		}
	}
	u.RawQuery = query.Encode()

	return u
}

func mustBeReader[T any](record T) io.Reader {
	data, err := json.Marshal(record)
	if err != nil {
		panic(err)
	}
	return bytes.NewReader(data)
}

func TestMakeJSONRequest(t *testing.T) {
	db, engine, err := basicSetup("TestMakeJSONRequest.db")
	if err != nil {
		t.Fatal(err)
	}

	err = Setup(engine.Group("/user"), db, nil, &Crud[User]{})
	if err != nil {
		t.Fatal(err)
	}

	var binding = "127.0.0.1:8020"
	var addr = "http://" + binding

	go func() {
		_ = engine.Run(binding)
	}()

	t.Logf("Server started on %s", binding)

	wait(t)

	// okay http status
	res := new(R[any])
	err = MakeJSONRequest(http.DefaultClient, &HttpStatusRange{100, 199}, mustBeURL(addr+"/user/one/1"), http.MethodGet, nil, res)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	t.Logf("%v", err)
	if !strings.HasPrefix(err.Error(), "status code: ") {
		t.Fatalf("expected error starts with [status code: ], but got %s", err.Error())
	}

	// json unmarshal error of type mismatch
	res1 := new(R[User])
	err = MakeJSONRequest(http.DefaultClient, nil, mustBeURL(addr+"/user/count"), http.MethodGet, nil, res1)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	t.Logf("%v", err)
	if !strings.HasPrefix(err.Error(), "unable to convert from ") {
		t.Fatalf("expected error starting with [unable to convert from ], but got %s", err.Error())
	}

	res2 := new(R[User])
	err = MakeJSONRequest(http.DefaultClient, nil, mustBeURL(addr+"/user"), http.MethodPut, mustBeReader(User{
		Name: "Tester1",
	}), res2)
	if err != nil {
		t.Fatal(err)
	} else if res2 == nil {
		t.Fatal("nil response")
	} else if res2.Data.ID != 1 {
		t.Fatalf("expected ID 1, got %d", res2.Data.ID)
	}

	res3 := new(R[[]User])
	err = MakeJSONRequest(http.DefaultClient, nil, mustBeURL(addr+"/user/page/1/10"), http.MethodPost, nil, res3)
	if err != nil {
		t.Fatal(err)
	} else if res3 == nil {
		t.Fatal("nil response")
	} else if len(res3.Data) != 1 {
		t.Fatalf("expected 1 user, got %d", len(res3.Data))
	}

	res4 := new(R[User])
	err = MakeJSONRequest(http.DefaultClient, nil, mustBeURL(addr+"/user/one/1"), http.MethodGet, nil, res4)
	if err != nil {
		t.Fatal(err)
	} else if res4 == nil {
		t.Fatal("nil response")
	} else if res4.Data.ID != 1 {
		t.Fatalf("expected ID 1, got %d", res4.Data.ID)
	}
}

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

	userAddr := "http://127.0.0.1:8021/user"

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
