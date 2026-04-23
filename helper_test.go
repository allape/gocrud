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

func TestStringArrayFromCommaSeparatedString(t *testing.T) {
	arr := StringArrayFromCommaSeparatedString("a,b,c")
	if len(arr) != 3 {
		t.Fatalf("expected 3, got %d", len(arr))
	}

	arr = StringArrayFromCommaSeparatedString(",,,   , , , ,   \r\n\r\n\n\n\n\n     ,,,,\t\t\t\t\t1,   2,   3       ,a,,,")
	if len(arr) != 4 {
		t.Fatalf("expected 4, got %d", len(arr))
	} else if arr[0] != "1" {
		t.Fatalf("expected 1, got %s", arr[0])
	} else if arr[1] != "2" {
		t.Fatalf("expected 2, got %s", arr[1])
	} else if arr[2] != "3" {
		t.Fatalf("expected 3, got %s", arr[2])
	} else if arr[3] != "a" {
		t.Fatalf("expected a, got %s", arr[3])
	}
}

func TestTernaryFunc(t *testing.T) {
	v := TernaryFunc(func() bool {
		return false
	}, func() int {
		return 1
	}, func() int {
		return 2
	})
	if v != 2 {
		t.Fatalf("expect 2, got %d", v)
	}

	v = TernaryFunc(func() bool {
		return true
	}, func() int {
		return 1
	}, func() int {
		return 2
	})
	if v != 1 {
		t.Fatalf("expect 1, got %d", v)
	}
}

func TestRemoveDuplication(t *testing.T) {
	a := []int{2, 3, 1, 2, 3, 3, 4, 5, 3, 4, 5, 1, 2, 3}
	b := RemoveDuplication(a)
	if len(b) != 5 {
		t.Fatalf("expected 5, got %d", len(b))
	}

	a = []int{1, 3, 5, 7, 9, 11}
	b = RemoveDuplication(a)
	if len(b) != 6 {
		t.Fatalf("expected 6, got %d", len(b))
	}
}
