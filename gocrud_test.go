package gocrud

import (
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"io"
	"log"
	"net/http"
	"os"
	"testing"
	"time"
)

const TestDBName = "test.db"

const (
	HttpBinding = "127.0.0.1:8080"
)

var HttpClient = &http.Client{}

type User struct {
	ID        uint64    `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name"`
	Deleted   bool      `json:"deleted" gorm:"default:false"`
	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"updatedAt" gorm:"autoUpdateTime"`
}

func StartServer(t *testing.T) *gin.Engine {
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
				log.Println("id filter:", value)
				return value
			}),
			"name": KeywordLike("name", nil),
			"deleted": KeywordEqual("deleted", func(value string) any {
				return value == "true"
			}),
		},
		OnDelete: func(ctx *gin.Context, repo *gorm.DB) (bool, error) {
			id := ctx.Query("id")
			if id == "" {
				return false, nil
			}

			result := repo.Model(&User{}).Where("id = ? and deleted = false", id).Update("deleted", true)
			return result.RowsAffected > 0, result.Error
		},
	}

	err = user.Setup("user", engine, db)
	if err != nil {
		t.Fatal(err)
	}

	return engine
}

func TestDefault(t *testing.T) {
	router := StartServer(t)

	go func() {
		_ = router.Run(HttpBinding)
	}()

	t.Log("Server started on port 8080")

	time.Sleep(time.Second)

	for i := 0; i < 3; i++ {
		t.Log(3-i, "...")
		time.Sleep(time.Second)
	}

	//goland:noinspection HttpUrlsUsage
	const AddrPrefix = "http://" + HttpBinding

	// test save
	u1, err := MakeJSONRequest[*User](http.MethodPut, AddrPrefix+"/user", User{Name: "test1"})
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

	_, err = MakeJSONRequest[User](http.MethodPut, AddrPrefix+"/user", User{Name: "test2"})
	if err != nil {
		t.Fatal(err)
	}

	// test get all
	all, err := MakeJSONRequest[[]User](http.MethodGet, AddrPrefix+"/user/all?name=test", nil)
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
	all, err = MakeJSONRequest[[]User](http.MethodGet, AddrPrefix+"/user/all?id=1&id=3&id=5", nil)
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
	page, err := MakeJSONRequest[[]User](http.MethodGet, AddrPrefix+"/user/1/1", nil)
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
	_, err = MakeJSONRequest[any](http.MethodDelete, AddrPrefix+"/user?id=1", nil)
	if err != nil {
		t.Fatal(err)
	}

	// test count
	count, err := MakeJSONRequest[uint64](http.MethodGet, AddrPrefix+"/user/count?deleted=false", nil)
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
	u1, err = MakeJSONRequest[*User](http.MethodGet, AddrPrefix+"/user?id=1", nil)
	if err != nil {
		t.Fatal(err)
	} else if u1 == nil || u1.Data == nil {
		t.Fatal("response is nil")
	} else if u1.Code != "0" {
		t.Fatal("response code is not 0")
	} else if !u1.Data.Deleted {
		t.Fatal("response data deleted is false")
	}
}

func MakeJSONRequest[T any](method, url string, body any) (*R[T], error) {
	var reader io.Reader

	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := HttpClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	var result R[T]
	err = json.NewDecoder(resp.Body).Decode(&result)

	return &result, err
}

// TestStartServer this will never pass
func TestStartServer(t *testing.T) {
	router := StartServer(t)
	_ = router.Run(HttpBinding)
}
