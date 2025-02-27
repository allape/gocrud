package gocrud

import (
	"bytes"
	crand "crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
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

func TestPick(t *testing.T) {
	arr := []int{1, 2, 3}
	if Pick(arr, 0, 0) != 1 {
		t.Fatal("Pick failed")
	} else if Pick(arr, 3, 0) != 0 {
		t.Fatal("Pick failed")
	} else if Pick(arr, 4, 0) != 0 {
		t.Fatal("Pick failed")
	}
}

func TestNowString(t *testing.T) {
	if ok, err := regexp.Match("^\\d{4}-\\d{2}-\\d{2} \\d{2}:\\d{2}:\\d{2}.\\d{3}$", []byte(NowString(nil))); !ok || err != nil {
		t.Fatal("NowString failed")
	}
}
