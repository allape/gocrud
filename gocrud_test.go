package gocrud

import (
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/http"
	"os"
	"testing"
)

const (
	TestDBName = "test.db"
)

type User struct {
	Base
	Name string `json:"name"`
}

func startServer(t *testing.T) (*gin.Engine, string) {
	engine := gin.New()

	engine.Use(RecoveryHandler(true))

	_, err := os.Stat(TestDBName)
	if err != nil {
		if !os.IsNotExist(err) {
			t.Fatal(err)
		}
	} else {
		err = os.Remove(TestDBName)
		if err != nil {
			t.Fatal(err)
		}
	}

	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{
		Logger: logger.Default,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = db.AutoMigrate(&User{})
	if err != nil {
		t.Fatal(err)
	}

	user := CRUD[User]{
		AllowAnyPageSize: true,
		EnableGetAll:     true,
		SearchHandlers: SearchHandlers{
			"createdAt": SortBy("created_at"),
			"id": KeywordIn("id", func(value []string) []string {
				t.Log("id filter:", value)
				return value
			}),
			"name":    KeywordLike("name", nil),
			"deleted": HandleSoftDeleteSearch,
		},
		OnDelete: NewSoftDeleteHandler[User](),
	}

	userGroup := engine.Group("/user")
	err = user.Setup(userGroup, db)
	if err != nil {
		t.Fatal(err)
	}

	return engine, "127.0.0.1:8080"
}

func TestDefault(t *testing.T) {
	router, binding := startServer(t)

	go func() {
		_ = router.Run(binding)
	}()

	t.Log("Server started on port 8080")

	Wait(t)

	//goland:noinspection HttpUrlsUsage
	var AddrPrefix = "http://" + binding

	// test save
	u1, err := fetch[*User](http.MethodPut, AddrPrefix+"/user", User{Name: "test1"})
	if err != nil {
		t.Fatal(err)
	} else if u1 == nil || u1.Data == nil {
		t.Fatal("response is nil")
	} else if u1.Code != "0" {
		t.Fatal("response code is not 0")
	} else if u1.Data.ID != 1 {
		t.Fatal("response data id is not 1")
	} else if u1.Data.Name != "test1" {
		t.Fatal("response data name is not test")
	}

	_, err = fetch[User](http.MethodPut, AddrPrefix+"/user", User{Name: "test2"})
	if err != nil {
		t.Fatal(err)
	}

	// test get all
	all, err := fetch[[]User](http.MethodGet, AddrPrefix+"/user/all?name=test", nil)
	if err != nil {
		t.Fatal(err)
	} else if all == nil {
		t.Fatal("response is nil")
	} else if all.Code != "0" {
		t.Fatal("response code is not 0")
	} else if len(all.Data) != 2 {
		t.Fatal("response data length is not 2")
	}

	// test get all with id filter
	all, err = fetch[[]User](http.MethodGet, AddrPrefix+"/user/all?id=1&id=3&id=5", nil)
	if err != nil {
		t.Fatal(err)
	} else if all == nil {
		t.Fatal("response is nil")
	} else if all.Code != "0" {
		t.Fatal("response code is not 0")
	} else if len(all.Data) != 1 {
		t.Fatal("response data length is not 1")
	}

	// test page
	page, err := fetch[[]User](http.MethodGet, AddrPrefix+"/user/1/1", nil)
	if err != nil {
		t.Fatal(err)
	} else if page == nil {
		t.Fatal("response is nil")
	} else if page.Code != "0" {
		t.Fatal("response code is not 0")
	} else if len(page.Data) != 1 {
		t.Fatal("response data length is not 1")
	} else if page.Data[0].Name != "test1" {
		t.Fatal("response data name is not test1")
	}

	// test delete
	_, err = fetch[any](http.MethodDelete, AddrPrefix+"/user?id=1", nil)
	if err != nil {
		t.Fatal(err)
	}

	// test count
	count, err := fetch[uint64](http.MethodGet, AddrPrefix+"/user/count?deleted=false", nil)
	if err != nil {
		t.Fatal(err)
	} else if count == nil {
		t.Fatal("response is nil")
	} else if count.Code != "0" {
		t.Fatal("response code is not 0")
	} else if count.Data != 1 {
		t.Fatal("response data count is not 1")
	}

	// test deleted user
	u1, err = fetch[*User](http.MethodGet, AddrPrefix+"/user?id=1", nil)
	if err != nil {
		t.Fatal(err)
	} else if u1 == nil || u1.Data == nil {
		t.Fatal("response is nil")
	} else if u1.Code != "0" {
		t.Fatal("response code is not 0")
	} else if u1.Data.DeletedAt == nil {
		t.Fatal("expected deleted user")
	}
}

// TestStartServer used by frontend testing
//
//goland:noinspection GoUnusedFunction
func _TestStartServer(t *testing.T) {
	router, binding := startServer(t)
	static := router.Group("/static")
	err := NewHttpFileSystem(static, TestData, &HttpFileSystemConfig{
		AllowOverwrite: true,
		AllowUpload:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = router.Run(binding)
}
