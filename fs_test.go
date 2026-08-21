package gocrud

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path"
	"testing"

	"github.com/gin-gonic/gin"
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

func TestNewHttpFileSystemController(t *testing.T) {
	var HttpBinding = address.fs.NewAddress(0)

	engine := gin.New()

	group := engine.Group("")

	var err error

	err = NewHttpFileSystemController(group, TestDataDir, &HttpFileSystemConfig{
		AllowUpload: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		_ = engine.Run(HttpBinding)
	}()

	wait(t)

	randomBytes, err := NewRandomBytes(10*MegaByte + rand.Intn(100)*MegaByte)
	if err != nil {
		t.Fatal(err)
	}

	var url string
	var result *R[any]

	url = "http://" + HttpBinding + TestFileName

	hash := sha256.Sum256(randomBytes)
	digest := hex.EncodeToString(hash[:])

	result, err = fetchJSON[any](http.MethodPost, url, bytes.NewReader(randomBytes), map[string]string{
		XFileDigest: digest + "123",
	})
	if err != nil {
		t.Fatal(err)
	} else if result.Code == RestCoder.OK() {
		t.Fatalf("response status is ok")
	} else if result.Message != ErrorFileDigestMismatch.Error() {
		t.Fatalf("response message is not %s, got %s", ErrorFileDigestMismatch.Error(), result.Message)
	}

	result, err = fetchJSON[any](http.MethodPost, url, bytes.NewReader(randomBytes), map[string]string{
		XFileDigest: digest,
	})
	if err != nil {
		t.Fatal(err)
	} else if result.Code != RestCoder.OK() {
		t.Fatalf("response status is not ok, got %s", result.Code)
	} else if fmt.Sprintf("%v", result.Data) == TestFileName {
		t.Fatalf("response data should not be %s", TestFileName)
	}

	result, err = fetchJSON[any](http.MethodPost, url, bytes.NewReader(randomBytes), nil)
	if err != nil {
		t.Fatal(err)
	} else if result.Code != RestCoder.OK() {
		t.Fatalf("response status is not ok, got %s", result.Code)
	}

	if ok, err := compareFileBytes(path.Join(TestDataDir, digest[:2], digest[2:4], digest+".mybin"), randomBytes); !ok || err != nil {
		t.Fatal("local bytes is not equal", err)
	}
}
