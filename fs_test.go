package gocrud

import (
	"bytes"
	"github.com/gin-gonic/gin"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path"
	"testing"
)

const (
	TestFileName = "/test1/test.bin"
)

func TestStaticServ(t *testing.T) {
	const HttpBinding = "127.0.0.1:8081"

	engine := gin.New()

	group1 := engine.Group("/static1")
	group2 := engine.Group("/static2")
	group3 := engine.Group("/static3")

	coder := RestCoder

	err := NewHttpFileSystem(group1, TestData, &HttpFileSystemConfig{
		AllowUpload:    true,
		AllowOverwrite: true,
		Coder:          coder,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = NewHttpFileSystem(group2, TestData, &HttpFileSystemConfig{
		AllowUpload:    true,
		AllowOverwrite: false,
		Coder:          coder,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = NewHttpFileSystem(group3, TestData, &HttpFileSystemConfig{
		AllowUpload:    false,
		AllowOverwrite: false,
		Coder:          coder,
	})
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		_ = engine.Run(HttpBinding)
	}()

	Wait(t)

	randomBytes, err := NewRandomBytes(10*MegaByte + rand.Intn(100)*MegaByte)
	if err != nil {
		t.Fatal(err)
	}

	//goland:noinspection HttpUrlsUsage
	url := "http://" + HttpBinding + "/static1" + TestFileName

	res, err := fetchJSON[any](http.MethodPost, url, bytes.NewReader(randomBytes), nil)
	if err != nil {
		t.Fatal(err)
	} else if res.Code != coder.OK() {
		t.Fatalf("response status is not ok, got %s", res.Code)
	}

	// validate
	bs, err := fetchBytes(http.MethodGet, url, nil, nil)
	if err != nil {
		t.Fatal(err)
	} else if bytes.Compare(bs, randomBytes) != 0 {
		t.Fatal("response bytes is not equal")
	}

	// validate local
	file, err := os.Open(path.Join(TestData, TestFileName))
	if err != nil {
		t.Fatal(err)
	}

	localBs := new(bytes.Buffer)
	_, err = io.Copy(localBs, file)
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Compare(localBs.Bytes(), randomBytes) != 0 {
		t.Fatal("local bytes is not equal")
	}

	// test not allowed to overwrite

	//goland:noinspection HttpUrlsUsage
	url = "http://" + HttpBinding + "/static2" + TestFileName

	result, err := fetchJSON[any](http.MethodPost, url, bytes.NewReader(randomBytes), nil)
	if err != nil {
		t.Fatal(err)
	} else if result.Code != coder.Conflict() {
		t.Fatalf("response status is not conflict, got %s", result.Code)
	}

	// test not allowed to upload

	//goland:noinspection HttpUrlsUsage
	url = "http://" + HttpBinding + "/static3" + TestFileName

	result, err = fetchJSON[any](http.MethodPost, url, bytes.NewReader(randomBytes), nil)
	if err != nil {
		t.Fatal(err)
	} else if result.Code != coder.MethodNotAllowed() {
		t.Fatalf("response status is not method not allowed, got %s", result.Code)
	}
}
