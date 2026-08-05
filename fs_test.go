package gocrud

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path"
	"sync"
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

func TestNewHttpFileSystem(t *testing.T) {
	const HttpBinding = "127.0.0.1:8081"

	engine := gin.New()

	group := engine.Group("")

	var err error

	err = NewHttpFileSystem(group, TestDataDir, &HttpFileSystemConfig{
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

func TestNewEncryptedHttpFileSystem(t *testing.T) {
	const HttpBinding = "127.0.0.1:8082"

	engine := gin.New()

	group := engine.Group("")

	var err error

	err = NewHttpFileSystem(group, TestDataDir, &HttpFileSystemConfig{
		AllowUpload:   true,
		FileMasterKey: masterKey,
	})
	if !errors.Is(err, ErrorFileKeyProviderIsNil) {
		t.Fatalf("expect ErrorFileKeyProviderIsNil, but got %v", err)
	}

	var dareFiles []*HttpFile
	var locker sync.Mutex

	err = NewHttpFileSystem(group, TestDataDir, &HttpFileSystemConfig{
		AllowUpload:   true,
		FileMasterKey: masterKey,
		OnFileSaved: func(file *HttpFile) {
			locker.Lock()
			defer locker.Unlock()

			t.Logf("new file %v", file)

			dareFiles = append(dareFiles, file)
		},
		FileKeyProvider: func(filename string) (Filename, FileSize, FileKey) {
			for _, dareFile := range dareFiles {
				if dareFile.Filename == Filename(filename) {
					return dareFile.Filename, dareFile.Length, dareFile.FileKey
				}
			}
			return "", 0, nil
		},
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
		XFileDigest: digest,
	})
	if err != nil {
		t.Fatal(err)
	} else if result.Code != RestCoder.OK() {
		t.Fatalf("response status is not ok, got %s: %s", result.Code, result.Message)
	}

	filename := result.Data.(string)

	same, err := compareFileBytes(path.Join(TestDataDir, result.Data.(string)), randomBytes)
	if err != nil {
		t.Fatal(err)
	} else if same {
		t.Fatalf("file content is same %s", result.Data.(string))
	}

	resp, err := http.DefaultClient.Get("http://" + HttpBinding + filename)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	} else if bytes.Compare(data, randomBytes) != 0 {
		t.Fatalf("file content is not same %s", result.Data.(string))
	}
}

//goland:noinspection GoUnusedFunction
func testEncryptedHttpFileSystem(t *testing.T) {
	const HttpBinding = "127.0.0.1:8080"

	engine := gin.New()

	var dareFiles []*HttpFile
	var locker sync.Mutex

	err := NewHttpFileSystem(engine.Group(""), TestDataDir, &HttpFileSystemConfig{
		AllowUpload:   true,
		FileMasterKey: masterKey,
		OnFileSaved: func(file *HttpFile) {
			locker.Lock()
			defer locker.Unlock()

			t.Logf("new file %s", file.Filename)

			dareFiles = append(dareFiles, file)
		},
		FileKeyProvider: func(filename string) (Filename, FileSize, FileKey) {
			for _, dareFile := range dareFiles {
				if dareFile.Filename == Filename(filename) {
					t.Logf("get file %v", dareFile.Filename)
					return dareFile.Filename, dareFile.Length, dareFile.FileKey
				}
			}
			return "", 0, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		_ = engine.Run(HttpBinding)
	}()

	Wait4CtrlC()
}
