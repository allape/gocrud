package gocrud

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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
	var HttpBinding = address.fsObject.NewAddress(0)

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
		OnFileReview: func(filename Filename) (*HttpFile, error) {
			for _, file := range savedFiles {
				if file.Filename == filename {
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
	var HttpBinding = address.fsObject.NewAddress(1)

	db, engine, err := basicSetup("TestNewHttpFileSystemObjectController.db")
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		_ = engine.Run(HttpBinding)
	}()

	t.Logf("Server started on %s", HttpBinding)

	wait(t)

	config := NewHttpFileSystemObjectConfig[DemoHttpFileObject](true, masterKey)

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

func TestNewHttpFileSystemObjectHandler(t *testing.T) {
	db, _, err := basicSetup("TestNewHttpFileSystemObjectHandler.db")
	if err != nil {
		t.Fatal(err)
	}

	handler, err := NewHttpFileSystemObjectHandler[DemoHttpFileObject](
		db, gogger.New("hfso:handler"),
		TestDataDir, masterKey, "",
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("%v", handler.BaseFolder)

	randomBytes, err := NewRandomBytes(10*MegaByte + rand.Intn(100)*MegaByte)
	if err != nil {
		t.Fatal(err)
	}

	hash := sha256.Sum256(randomBytes)
	digest := hex.EncodeToString(hash[:])

	httpFile, err := handler.Save(bytes.NewReader(randomBytes), ".bin", FileSize(len(randomBytes)), FileDigest(digest))
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("%v", httpFile)

	_, err = os.Stat(path.Join(TestDataDir, string(httpFile.Filename)))
	if err != nil {
		t.Fatal(err)
	}

	buffer := bytes.NewBuffer(nil)
	reader, err := handler.NewReader(string(httpFile.Filename))
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.Copy(buffer, reader)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(buffer.Bytes(), randomBytes) {
		t.Fatalf("file content is not same %s", buffer.String())
	}
}

//goland:noinspection GoUnusedFunction
func testRunEncryptedHttpFileSystem(t *testing.T) {
	var HttpBinding = address.fsObject.NewAddress(2)

	engine := gin.New()

	var savedFiles []*HttpFile
	var locker sync.Mutex

	err := NewHttpFileSystemController(engine.Group(""), TestDataDir, &HttpFileSystemConfig{
		AllowUpload:   true,
		FileMasterKey: masterKey,
		OnFileReview: func(filename Filename) (*HttpFile, error) {
			for _, file := range savedFiles {
				if file.Filename == filename {
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
