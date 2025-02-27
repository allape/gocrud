package gocrud

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path"
	"testing"
)

const (
	TestFileName = "/test1/test.mybin"
)

func compareFileBytes(filename string, byteArray []byte) (bool, error) {
	file, err := os.Open(filename)
	if err != nil {
		return false, err
	}
	defer func() {
		_ = file.Close()
	}()

	localBs := new(bytes.Buffer)
	_, err = io.Copy(localBs, file)
	if err != nil {
		return false, err
	}

	return bytes.Compare(localBs.Bytes(), byteArray) == 0, nil
}

//goland:noinspection HttpUrlsUsage
func TestStaticServ(t *testing.T) {
	const HttpBinding = "127.0.0.1:8081"

	engine := gin.New()

	group1 := engine.Group("/static1")
	group2 := engine.Group("/static2")
	group3 := engine.Group("/static3")
	group4 := engine.Group("/static4")

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

	err = NewHttpFileSystem(group4, TestData, &HttpFileSystemConfig{
		AllowUpload:        true,
		AllowOverwrite:     true,
		EnableServerDigest: true,
		Coder:              coder,
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
	if ok, err := compareFileBytes(path.Join(TestData, TestFileName), randomBytes); !ok || err != nil {
		t.Fatal("local bytes is not equal", err)
	}

	// test not allowed to overwrite

	url = "http://" + HttpBinding + "/static2" + TestFileName

	result, err := fetchJSON[any](http.MethodPost, url, bytes.NewReader(randomBytes), nil)
	if err != nil {
		t.Fatal(err)
	} else if fmt.Sprintf("%v", result.Data) != TestFileName {
		t.Fatalf("response data is not %s, got %s", TestFileName, result.Data)
	}

	// test not allowed to upload

	url = "http://" + HttpBinding + "/static3" + TestFileName

	result, err = fetchJSON[any](http.MethodPost, url, bytes.NewReader(randomBytes), nil)
	if err != nil {
		t.Fatal(err)
	} else if result.Code != coder.MethodNotAllowed() {
		t.Fatalf("response status is not method not allowed, got %s", result.Code)
	}

	// test server digest

	url = "http://" + HttpBinding + "/static4" + TestFileName

	hash := sha256.Sum256(randomBytes)
	digest := hex.EncodeToString(hash[:])

	result, err = fetchJSON[any](http.MethodPost, url, bytes.NewReader(randomBytes), map[string]string{
		XFileDigest: digest + "123",
	})
	if err != nil {
		t.Fatal(err)
	} else if result.Code == coder.OK() {
		t.Fatalf("response status is ok")
	} else if result.Message != ErrorFileDigestMismatch.Error() {
		t.Fatalf("response message is not %s, got %s", ErrorFileDigestMismatch.Error(), result.Message)
	}

	result, err = fetchJSON[any](http.MethodPost, url, bytes.NewReader(randomBytes), map[string]string{
		XFileDigest: digest,
	})
	if err != nil {
		t.Fatal(err)
	} else if result.Code != coder.OK() {
		t.Fatalf("response status is not ok, got %s", result.Code)
	} else if fmt.Sprintf("%v", result.Data) == TestFileName {
		t.Fatalf("response data should not be %s", TestFileName)
	}

	result, err = fetchJSON[any](http.MethodPost, url, bytes.NewReader(randomBytes), nil)
	if err != nil {
		t.Fatal(err)
	} else if result.Code != coder.OK() {
		t.Fatalf("response status is not ok, got %s", result.Code)
	}

	if ok, err := compareFileBytes(path.Join(TestData, digest[:2], digest[2:4], digest+".mybin"), randomBytes); !ok || err != nil {
		t.Fatal("local bytes is not equal", err)
	}
}
