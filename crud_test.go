package gocrud

import (
	"bytes"
	"encoding/base64"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	censored "github.com/allape/gocensored"
	"github.com/allape/gogger"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	TestDBName = "test.db"
)

type User struct {
	Base
	Name string `json:"name"`
	Age  int    `json:"age"`
}

type Tag struct {
	Base
	Name string `json:"name"`
}

type UserTag struct {
	UserID ID `json:"userId,omitempty" gorm:"primaryKey"`
	TagID  ID `json:"tagId" gorm:"primaryKey"`
}

type SecretUser struct {
	Base
	Name string `json:"name" censored:"aes.base64"`
}

func basicSetup() (*gorm.DB, *gin.Engine, error) {
	_, err := os.Stat(TestDBName)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, nil, err
		}
	} else {
		err = os.Remove(TestDBName)
		if err != nil {
			return nil, nil, err
		}
	}

	db, err := gorm.Open(sqlite.Open(TestDBName), &gorm.Config{
		Logger: logger.New(gogger.New("database").Info(), logger.Config{
			SlowThreshold: 200 * time.Millisecond,
			LogLevel:      logger.Info,
			Colorful:      true,
		}),
	})
	if err != nil {
		return nil, nil, err
	}

	err = db.AutoMigrate(
		&User{},
		&SecretUser{},
		&Tag{},
		&UserTag{},
	)
	if err != nil {
		return nil, nil, err
	}

	engine := gin.New()
	engine.Use(RecoveryHandler(true))
	engine.Use(NewCors())

	return db, engine, nil
}

func TestSetup(t *testing.T) {
	db, engine, err := basicSetup()
	if err != nil {
		t.Fatal(err)
	}

	err = Setup(nil, db, nil, &Crud[User]{})
	if !errors.Is(err, NilGroupError) {
		t.Fatalf("expecting NilGroupError, got %v", err)
		return
	}

	err = Setup(engine.Group("/user"), nil, nil, &Crud[User]{})
	if !errors.Is(err, NilDatabaseError) {
		t.Fatal("expected error")
	}
}

func TestNormalUser(t *testing.T) {
	db, engine, err := basicSetup()
	if err != nil {
		t.Fatal(err)
	}

	userL := gogger.New("controller:user")
	userCrudL := userL.New("crud")

	err = Setup(engine.Group("/user"), db, userCrudL, &Crud[User]{
		EnableGetAll: true,
		SearchHandlers: BaseSearchHandlers(SearchHandlers{
			"id": KeywordIn("id", func(value []string) []string {
				t.Log("id filter:", value)
				return value
			}),
			"field_not_found": KeywordLike("field_not_found", nil),
			"like_name":       KeywordLike("name", nil),
			"name":            KeywordLike("name", nil),
			"name_eq":         KeywordEqual("name", nil),
			"age_gte":         KeywordStatement("age", OperatorGte, NumericValidate),
		}),
		WillSave: func(record *User, context *gin.Context, db *gorm.DB) {
			if strings.Contains(record.Name, "freak") {
				MakeErrorResponse(context, RestCoder.BadRequest(), "freak is not allowed")
				return
			}

			if err := DuplicateFieldCheck[User](
				db, context, userCrudL, record,
				"Name", "name",
			); err != nil {
				return
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	var binding = "127.0.0.1:8001"
	var addr = "http://" + binding

	go func() {
		_ = engine.Run(binding)
	}()

	t.Logf("Server started on %s", binding)

	wait(t)

	crudy, err := NewCrudy[User](
		addr+"/user",
		CrudyPageOptions[User]{
			DefaultSize: DefaultPageSize,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = crudy.All(SearchParams{
		"field_not_found": "test",
	})
	if err == nil {
		t.Fatal("expected error")
	}

	// test save
	u1, err := crudy.Save(&User{Name: "test1", Age: 10})
	if err != nil {
		t.Fatal(err)
	} else if u1 == nil {
		t.Fatal("user1 is nil")
	} else if u1.ID != 1 {
		t.Fatal("user1's id is not 1")
	} else if u1.Name != "test1" {
		t.Fatal("user1's name is not test1")
	}

	u2, err := crudy.Save(&User{Name: "test2", Age: 9})
	if err != nil {
		t.Fatal(err)
	} else if u2 == nil {
		t.Fatal("user2 is nil")
	} else if u2.ID != 2 {
		t.Fatal("user2's id is not 2")
	} else if u2.Name != "test2" {
		t.Fatal("user2's name is not test2")
	}

	// test save with invalid name
	uFreak, err := crudy.Save(&User{Name: "freak", Age: 10})
	if err == nil {
		t.Fatal("expected error")
	} else if uFreak != nil {
		t.Fatal("user freak is not nil")
	}

	// test get all
	all, err := crudy.All(SearchParams{
		"name":  "test",
		"in_id": "1,,,23,4,5,6,2,3,4,",
	})
	if err != nil {
		t.Fatal(err)
	} else if all == nil {
		t.Fatal("response is nil")
	} else if len(all) != 2 {
		t.Fatal("length is not 2")
	}

	// test KeywordStatement
	all, err = crudy.All(SearchParams{
		"age_gte": "10",
	})
	if err != nil {
		t.Fatal(err)
	} else if all == nil {
		t.Fatal("response is nil")
	} else if len(all) != 1 {
		t.Fatal("length is not 1")
	}

	// test KeywordStatement
	all, err = crudy.All(SearchParams{
		"age_gte": "abc",
	})
	if err != nil {
		t.Fatal(err)
	} else if all == nil {
		t.Fatal("response is nil")
	} else if len(all) != 2 {
		t.Fatal("length is not 2")
	}

	// test get all with id filter
	all, err = crudy.All(SearchParams{
		"id": "1,3,5",
	})
	if err != nil {
		t.Fatal(err)
	} else if all == nil {
		t.Fatal("response is nil")
	} else if len(all) != 1 {
		t.Fatal("length is not 1")
	}

	// test page
	page, err := crudy.Page(1, 1, nil)
	if err != nil {
		t.Fatal(err)
	} else if page == nil {
		t.Fatal("response is nil")
	} else if len(page) != 1 {
		t.Fatal("length is not 1")
	} else if page[0].Name != "test1" {
		t.Fatal("the first name is not test1")
	}

	// test zero page
	page, err = crudy.Page(0, 0, nil)
	if err != nil {
		t.Fatal(err)
	} else if page == nil {
		t.Fatal("response is nil")
	} else if len(page) == 0 {
		t.Fatal("length should not be 0")
	}

	// test update
	time.Sleep(time.Second)
	editedU1, err := crudy.Save(&User{Base: Base{ID: 1}, Name: "test1", Age: 12})
	if err != nil {
		t.Fatal(err)
	} else if editedU1 == nil {
		t.Fatal("response is nil")
	} else if editedU1.ID != 1 {
		t.Fatal("id is not 1")
	}

	editedU1, err = crudy.One(1)
	if err != nil {
		t.Fatal(err)
	} else if editedU1 == nil {
		t.Fatal("response is nil")
	} else if editedU1.CreatedAt.Unix() != u1.CreatedAt.Unix() {
		t.Fatal("created_at is not equal")
	} else if editedU1.UpdatedAt.Unix() == u1.UpdatedAt.Unix() {
		t.Fatal("updated_at is equal")
	}

	// test delete
	deleted, err := crudy.Delete(0)
	if err != nil {
		t.Fatal(err)
	} else if deleted {
		t.Fatal("response is not false")
	}

	deleted, err = crudy.Delete(1)
	if err != nil {
		t.Fatal(err)
	} else if !deleted {
		t.Fatal("response is not true")
	}

	// test count
	count, err := crudy.Count(SearchParams{
		"deleted": "false",
	})
	if err != nil {
		t.Fatal(err)
	} else if count != 1 {
		t.Fatal("response data count is not 1")
	}

	// test deleted user
	u1, err = crudy.One(1)
	if err != nil {
		t.Fatal(err)
	} else if u1 == nil {
		t.Fatal("response is nil")
	} else if u1.DeletedAt == nil {
		t.Fatal("expected deleted user")
	}
}

func TestHardDelete(t *testing.T) {
	db, engine, err := basicSetup()
	if err != nil {
		t.Fatal(err)
	}

	err = Setup(engine.Group("/user"), db, nil, &Crud[User]{
		OnDelete: NewHardDeleteHandler[User](RestCoder),
	})
	if err != nil {
		t.Fatal(err)
	}

	var binding = "127.0.0.1:8002"
	var addr = "http://" + binding

	go func() {
		_ = engine.Run(binding)
	}()

	t.Logf("Server started on %s", binding)

	wait(t)

	crudy, err := NewCrudy[User](addr + "/user")
	if err != nil {
		t.Fatal(err)
	}

	_, err = crudy.Save(&User{Name: "test1"})
	if err != nil {
		t.Fatal(err)
	}

	deleted, err := crudy.Delete(1)
	if err != nil {
		t.Fatal(err)
	} else if !deleted {
		t.Fatal("response is not true")
	}

	_, err = crudy.One(1)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSecretUser(t *testing.T) {
	db, engine, err := basicSetup()
	if err != nil {
		t.Fatal(err)
	}

	censor, err := censored.NewDefaultCensor(&censored.Config{
		Password: []byte("123456789_0"),
	})
	if err != nil {
		t.Fatal(err)
	}

	err = Setup(engine.Group("/user"), db, nil, &Crud[SecretUser]{
		EnableGetAll: true,
		GetCensors: func(_ *gin.Context, _ *gorm.DB) ([]*censored.Censor, error) {
			return []*censored.Censor{censor}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = Setup(engine.Group("/user-not-decoded"), db, nil, &Crud[SecretUser]{})
	if err != nil {
		t.Fatal(err)
	}

	var binding = "127.0.0.1:8003"
	var addr = "http://" + binding

	go func() {
		_ = engine.Run(binding)
	}()

	t.Logf("Server started on %s", binding)

	wait(t)

	crudy, err := NewCrudy[SecretUser](addr + "/user")
	if err != nil {
		t.Fatal(err)
	}

	crudyNotDecoded, err := NewCrudy[SecretUser](addr + "/user-not-decoded")
	if err != nil {
		t.Fatal(err)
	}

	_, err = crudy.Save(&SecretUser{
		Name: "I am a freak",
	})
	if err != nil {
		t.Fatal(err)
	}

	freak, err := crudy.One(1)
	if err != nil {
		t.Fatal(err)
	} else if freak == nil {
		t.Fatal("freak is nil")
	} else if !strings.Contains(freak.Name, "freak") {
		t.Fatal("freak is not decensored")
	}

	freaks, err := crudy.Page(1, 10, nil)
	if err != nil {
		t.Fatal(err)
	} else if len(freaks) != 1 {
		t.Fatal("freaks length is not 1")
	}

	freaks, err = crudy.All(nil)
	if err != nil {
		t.Fatal(err)
	} else if len(freaks) != 1 {
		t.Fatal("freaks length is not 1")
	}

	publicFreak, err := crudyNotDecoded.One(1)
	if err != nil {
		t.Fatal(err)
	} else if publicFreak == nil {
		t.Fatal("public freak is nil")
	} else if publicFreak.Name == freak.Name {
		t.Fatal("public freak is not censored")
	}

	freakName, err := base64.StdEncoding.DecodeString(publicFreak.Name)
	if err != nil {
		t.Fatal(err)
	} else if bytes.Compare(freakName, []byte(freak.Name)) == 0 {
		t.Fatal("public freak is not censored")
	}
}

// TestStartServer used by frontend testing
//
//goland:noinspection GoUnusedFunction
func testStartServer(t *testing.T) {
	db, engine, err := basicSetup()
	if err != nil {
		t.Fatal(err)
	}

	err = Setup(engine.Group("/user"), db, nil, &Crud[User]{
		EnableGetAll: true,
		SearchHandlers: BaseSearchHandlers(SearchHandlers{
			"in_id":     KeywordIDIn("id", nil),
			"like_name": KeywordLike("name", nil),
			"name":      KeywordEqual("name", nil),
		}),
	})
	if err != nil {
		t.Fatal(err)
	}

	static := engine.Group("/static")
	err = NewHttpFileSystem(static, TestData, &HttpFileSystemConfig{
		AllowOverwrite: true,
		AllowUpload:    true,
	})
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		_ = engine.Run("127.0.0.1:8080")
	}()

	Wait4CtrlC()
}
