package gocrud

import (
	"bytes"
	crand "crypto/rand"
	"crypto/sha256"
	"github.com/gin-gonic/gin"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path"
	"testing"
	"time"
)

const (
	TestData     = "testdata"
	MegaByte     = 1024 * 1024
	TestFileName = "/test1/test.bin"
)

func TestStaticServ(t *testing.T) {
	engine := gin.New()

	group1 := engine.Group("/static1")
	group2 := engine.Group("/static2")

	err := NewHttpFileSystem(group1, TestData, HttpFileSystemConfig{
		AllowOverwrite: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = NewHttpFileSystem(group2, TestData, HttpFileSystemConfig{
		AllowOverwrite: false,
	})
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		_ = engine.Run(HttpBinding)
	}()

	for i := 0; i < 3; i++ {
		t.Log(3-i, "...")
		time.Sleep(time.Second)
	}

	randomBytes := make([]byte, 10*MegaByte+rand.Intn(100)*MegaByte)
	n, err := crand.Read(randomBytes)
	if err != nil {
		t.Fatal(err)
	} else if n != len(randomBytes) {
		t.Fatal("read bytes length is not equal")
	}

	originalHashBytes := sha256.Sum256(randomBytes)

	//goland:noinspection HttpUrlsUsage
	url := "http://" + HttpBinding + "/static1" + TestFileName

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(randomBytes))
	if err != nil {
		t.Fatal(err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	} else if res.StatusCode != http.StatusOK {
		t.Fatalf("response status is not ok, got %d", res.StatusCode)
	}

	// validate
	req, err = http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}

	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	} else if res.StatusCode != http.StatusOK {
		t.Fatalf("response status is not ok, got %d", res.StatusCode)
	}

	newHasher := sha256.New()
	n1, err := io.Copy(newHasher, res.Body)
	if err != nil {
		t.Fatal(err)
	} else if n1 != int64(len(randomBytes)) {
		t.Fatal("read bytes length is not equal")
	}

	newHashBytes := newHasher.Sum(nil)

	if bytes.Compare(originalHashBytes[:], newHashBytes) != 0 {
		t.Fatal("hash is not equal")
	}

	// validate local
	file, err := os.Open(path.Join(TestData, TestFileName))
	if err != nil {
		t.Fatal(err)
	}

	newHasher.Reset()
	n2, err := io.Copy(newHasher, file)
	if err != nil {
		t.Fatal(err)
	} else if n2 != int64(len(randomBytes)) {
		t.Fatal("read bytes length is not equal")
	}

	newHashBytes = newHasher.Sum(nil)
	if bytes.Compare(originalHashBytes[:], newHashBytes) != 0 {
		t.Fatal("hash is not equal")
	}

	// test not allowed to overwrite

	//goland:noinspection HttpUrlsUsage
	url = "http://" + HttpBinding + "/static2" + TestFileName

	req, err = http.NewRequest(http.MethodPost, url, bytes.NewReader(randomBytes))
	if err != nil {
		t.Fatal(err)
	}
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	} else if res.StatusCode != http.StatusConflict {
		t.Fatalf("response status is not conflict, got %d", res.StatusCode)
	}
}
