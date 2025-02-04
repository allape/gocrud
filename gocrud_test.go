package gocrud

import (
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log"
	"net/http"
	"os"
	"testing"
	"time"
)

const (
	TestDBName = "test.db"
)

type User struct {
	Base
	Name string `json:"name"`
	Age  int    `json:"age"`
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
		Logger: logger.New(log.New(os.Stdout, "\r\n", log.LstdFlags), logger.Config{
			SlowThreshold: 200 * time.Millisecond,
			LogLevel:      logger.Info,
			Colorful:      true,
		}),
	})
	if err != nil {
		t.Fatal(err)
	}

	err = db.AutoMigrate(&User{})
	if err != nil {
		t.Fatal(err)
	}

	err = New(engine.Group("/user"), db, CRUD[User]{
		EnableGetAll: true,
		SearchHandlers: SearchHandlers{
			"createdAt": SortBy("created_at"),
			"id": KeywordIn("id", func(value []string) []string {
				t.Log("id filter:", value)
				return value
			}),
			"name":    KeywordLike("name", nil),
			"name_eq": KeywordEqual("name", nil),
			"deleted": HandleSoftDeleteSearch,
			"age_gte": KeywordStatement("age", OperatorGte, NumericValidate),
		},
		OnDelete: NewSoftDeleteHandler[User](RestCoder),
	})
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
	u1, err := fetch[*User](http.MethodPut, AddrPrefix+"/user", User{Name: "test1", Age: 10})
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

	u2, err := fetch[User](http.MethodPut, AddrPrefix+"/user", User{Name: "test2", Age: 9})
	if err != nil {
		t.Fatal(err)
	} else if u2 == nil {
		t.Fatal("response is nil")
	} else if u2.Code != "0" {
		t.Fatal("response code is not 0")
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

	// test KeywordStatement
	all, err = fetch[[]User](http.MethodGet, AddrPrefix+"/user/all?age_gte=10", nil)
	if err != nil {
		t.Fatal(err)
	} else if all == nil {
		t.Fatal("response is nil")
	} else if all.Code != "0" {
		t.Fatal("response code is not 0")
	} else if len(all.Data) != 1 {
		t.Fatal("response data length is not 1")
	}

	// test KeywordStatement
	all, err = fetch[[]User](http.MethodGet, AddrPrefix+"/user/all?age_gte=abc", nil)
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
	all, err = fetch[[]User](http.MethodGet, AddrPrefix+"/user/all?id=1,3,5", nil)
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
	page, err := fetch[[]User](http.MethodGet, AddrPrefix+"/user/page/1/1", nil)
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

	// test update
	time.Sleep(time.Second)
	editedU1, err := fetch[*User](http.MethodPut, AddrPrefix+"/user", User{Base: Base{ID: 1}, Name: "test1", Age: 12})
	if err != nil {
		t.Fatal(err)
	} else if editedU1 == nil || editedU1.Data == nil {
		t.Fatal("response is nil")
	} else if editedU1.Code != "0" {
		t.Fatal("response code is not 0")
	} else if editedU1.Data.ID != 1 {
		t.Fatal("response data id is not 1")
	}

	editedU1, err = fetch[*User](http.MethodGet, AddrPrefix+"/user/one/1", nil)
	if err != nil {
		t.Fatal(err)
	} else if editedU1 == nil || editedU1.Data == nil {
		t.Fatal("response is nil")
	} else if editedU1.Data.CreatedAt.Unix() != u1.Data.CreatedAt.Unix() {
		t.Fatal("response data created_at is not equal")
	} else if editedU1.Data.UpdatedAt.Unix() == u1.Data.UpdatedAt.Unix() {
		t.Fatal("response data updated_at is equal")
	}

	// test delete
	deleted, err := fetch[bool](http.MethodDelete, AddrPrefix+"/user/1", nil)
	if err != nil {
		t.Fatal(err)
	} else if deleted.Code != "0" {
		t.Fatalf("expected code 0, got %s", deleted.Code)
	} else if !deleted.Data {
		t.Fatal("response is not true")
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
	u1, err = fetch[*User](http.MethodGet, AddrPrefix+"/user/one/1", nil)
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
