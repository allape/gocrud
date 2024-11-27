package gocrud

import (
	"bytes"
	"fmt"
	"github.com/gin-gonic/gin"
	mrand "math/rand"
	"net/http"
	"os"
	"path"
	"testing"
)

func TestNewSingleHTMLServe(t *testing.T) {
	const HttpBinding = "127.0.0.1:8082"

	err := os.MkdirAll(TestData, 0755)
	if err != nil {
		t.Fatal(err)
	}

	indexPath := path.Join(TestData, "index.html")

	file, err := os.Create(indexPath)
	if err != nil {
		t.Fatal(err)
	}

	random, err := NewRandomBytes((mrand.Intn(2) + 3) * MegaByte)
	if err != nil {
		t.Fatal("failed to create random bytes:", err)
	}

	_, err = file.Write(random)

	router := gin.New()

	uiGroup := router.Group("ui")
	err = NewSingleHTMLServe(uiGroup, indexPath, &SingleHTMLServeConfig{
		AllowReplace: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		_ = router.Run(HttpBinding)
	}()

	Wait(t)

	//goland:noinspection HttpUrlsUsage
	url := fmt.Sprintf("http://%s/ui/index.html", HttpBinding)

	bs, err := fetchBytes(http.MethodGet, url, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Compare(bs, random) != 0 {
		t.Fatal("content is not equal")
	}

	random, err = NewRandomBytes((mrand.Intn(2) + 3) * MegaByte)
	if err != nil {
		t.Fatal("failed to create random bytes:", err)
	}

	result, err := fetchJSON[any](http.MethodPut, url, bytes.NewReader(random), nil)
	if err != nil {
		t.Fatal(err)
	}

	if result.Code != "0" {
		t.Fatalf("expected code 0, got %s", result.Code)
	}

	bs, err = fetchBytes(http.MethodGet, url, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Compare(bs, random) != 0 {
		t.Fatal("content is not equal")
	}
}
