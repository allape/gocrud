package gocrud

import (
	"bytes"
	crand "crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

const (
	TestData = "testdata"
	MegaByte = 1024 * 1024
)

var HttpClient = &http.Client{}

func NewRandomBytes(size int) ([]byte, error) {
	random := make([]byte, size)
	n, err := crand.Read(random)
	if err != nil {
		return nil, err
	}
	random = random[:n]
	if len(random) == 0 {
		return nil, errors.New("random is empty")
	}
	return random, nil
}

func Wait(t *testing.T) {
	for i := 3; i > 0; i-- {
		t.Log(i, "...")
		time.Sleep(time.Second)
	}
}

func fetchBytes(method, url string, reader io.Reader, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := HttpClient.Do(req)
	if err != nil {
		return nil, err
	} else if resp.StatusCode != 200 {
		return nil, fmt.Errorf("expected status code 200, got %d", resp.StatusCode)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func fetchJSON[T any](method, url string, reader io.Reader, headers map[string]string) (*R[T], error) {
	bs, err := fetchBytes(method, url, reader, headers)
	if err != nil {
		return nil, err
	}

	var result R[T]
	err = json.Unmarshal(bs, &result)

	return &result, err
}

func fetch[T any](method, url string, body any) (*R[T], error) {
	var reader io.Reader

	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
	}

	return fetchJSON[T](method, url, reader, map[string]string{
		"Content-Type": "application/json",
	})
}
