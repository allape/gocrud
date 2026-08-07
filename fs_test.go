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

	"github.com/allape/gogger"
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
	const HttpBinding = "127.0.0.1:8030"

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

func testEncryptedHttpFile(t *testing.T, binding string, uriPrefix string) {
	var result *R[any]

	randomBytes, err := NewRandomBytes(10*MegaByte + rand.Intn(100)*MegaByte)
	if err != nil {
		t.Fatal(err)
	}

	hash := sha256.Sum256(randomBytes)
	digest := hex.EncodeToString(hash[:])

	result, err = fetchJSON[any](http.MethodPost, "http://"+binding+uriPrefix+TestFileName, bytes.NewReader(randomBytes), map[string]string{
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

	data, err := fetchBytes(http.MethodGet, "http://"+binding+uriPrefix+filename, nil, nil)
	if err != nil {
		t.Fatal(err)
	} else if bytes.Compare(data, randomBytes) != 0 {
		t.Fatalf("file content is not same %s", result.Data.(string))
	}
}

func TestNewEncryptedHttpFileSystem(t *testing.T) {
	const HttpBinding = "127.0.0.1:8031"

	engine := gin.New()

	group := engine.Group("")

	var err error

	err = NewHttpFileSystemController(group, TestDataDir, &HttpFileSystemConfig{
		AllowUpload:   true,
		FileMasterKey: masterKey,
	})
	if !errors.Is(err, ErrorFileKeyProviderIsNil) {
		t.Fatalf("expect ErrorFileKeyProviderIsNil, but got %v", err)
	}

	var savedFiles []*HttpFile
	var locker sync.Mutex

	err = NewHttpFileSystemController(group, TestDataDir, &HttpFileSystemConfig{
		AllowUpload:   true,
		FileMasterKey: masterKey,
		OnFileReview: func(filename string) (*HttpFile, error) {
			for _, file := range savedFiles {
				if file.Filename == Filename(filename) {
					return file, nil
				}
			}
			return nil, nil
		},
		OnFileSaved: func(file *HttpFile) error {
			locker.Lock()
			defer locker.Unlock()

			t.Logf("new file %v", file)

			savedFiles = append(savedFiles, file)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		_ = engine.Run(HttpBinding)
	}()

	wait(t)

	testEncryptedHttpFile(t, HttpBinding, "")
}

type DemoHttpFileObject struct {
	HttpFileSystemObjectBase
}

func TestNewHttpFileSystemObjectController(t *testing.T) {
	const HttpBinding = "127.0.0.1:8032"

	db, engine, err := basicSetup("TestNewHttpFileSystemObjectController.db")
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		_ = engine.Run(HttpBinding)
	}()

	t.Logf("Server started on %s", HttpBinding)

	wait(t)

	config := &HttpFileSystemConfig{
		AllowUpload:   true,
		FileMasterKey: masterKey,
	}

	err = NewHttpFileSystemObjectController[DemoHttpFileObject](
		engine.Group(""), db, gogger.New("oss"),
		TestDataDir, config,
		"FieldNotExists",
	)
	if err == nil {
		t.Fatal("expect baseStructField not found, but got nil")
	}

	err = NewHttpFileSystemObjectController[DemoHttpFileObject](
		engine.Group("/extended"), db, gogger.New("oss:extended"),
		TestDataDir, config,
		"HttpFileSystemObjectBase",
	)
	if err != nil {
		t.Fatal(err)
	}

	testEncryptedHttpFile(t, HttpBinding, "/extended")
}

//goland:noinspection GoUnusedFunction
func testRunEncryptedHttpFileSystem(t *testing.T) {
	const HttpBinding = "127.0.0.1:8080"

	engine := gin.New()

	var savedFiles []*HttpFile
	var locker sync.Mutex

	err := NewHttpFileSystemController(engine.Group(""), TestDataDir, &HttpFileSystemConfig{
		AllowUpload:   true,
		FileMasterKey: masterKey,
		OnFileReview: func(filename string) (*HttpFile, error) {
			for _, file := range savedFiles {
				if file.Filename == Filename(filename) {
					return file, nil
				}
			}
			return nil, nil
		},
		OnFileSaved: func(file *HttpFile) error {
			locker.Lock()
			defer locker.Unlock()

			t.Logf("new file %v", file)

			savedFiles = append(savedFiles, file)
			return nil
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
