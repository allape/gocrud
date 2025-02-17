package gocrud

import (
	"errors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http"
	"net/url"
	"slices"
	"strconv"
)

var (
	NilGroupError      = errors.New("engine is nil")
	NilRepositoryError = errors.New("database is nil")
)

var (
	DefaultPageSizes = []int64{10, 20, 50, 100}
	DefaultPageSize  = DefaultPageSizes[0]
)

type (
	SearchHandler  = func(db *gorm.DB, values []string, with url.Values) *gorm.DB
	SearchHandlers = map[string]SearchHandler
)

type CRUD[T any] struct {
	DisallowAnyPageSize bool
	DefaultPageSize     int64
	PageSizes           []int64

	SearchHandlers SearchHandlers

	EnableGetAll  bool
	DisableGetOne bool
	DisableCount  bool
	DisablePage   bool
	DisableSave   bool
	DisableDelete bool

	// Callback func starts with `On` will replace the default operation,
	//               starts with `Will` will be called before the default operation,
	// 	             starts with `Did` will be called after the default operation.

	WillGetAll func(context *gin.Context, db *gorm.DB) *gorm.DB
	DidGetAll  func(records []T, context *gin.Context, db *gorm.DB)

	WillGetOne func(context *gin.Context, db *gorm.DB) *gorm.DB
	DidGetOne  func(record *T, context *gin.Context, db *gorm.DB)

	WillCount func(context *gin.Context, db *gorm.DB) *gorm.DB
	DidCount  func(count *int64, context *gin.Context, db *gorm.DB)

	WillPage func(pageNum *int64, pageSize *int64, context *gin.Context, db *gorm.DB) *gorm.DB
	DidPage  func(pageNum int64, pageSize int64, list []T, context *gin.Context, db *gorm.DB)

	WillSave func(record *T, context *gin.Context, db *gorm.DB)
	DidSave  func(record *T, context *gin.Context, db *gorm.DB)

	WillDelete func(context *gin.Context, db *gorm.DB)
	OnDelete   func(context *gin.Context, db *gorm.DB) bool
	DidDelete  func(context *gin.Context, db *gorm.DB)

	Coder             Coder
	MakeOkResponse    func(context *gin.Context, data any)
	MakeErrorResponse func(context *gin.Context, code Code, err error)

	group    *gin.RouterGroup
	database *gorm.DB
}

func (crud *CRUD[T]) makeOne() *T {
	return new(T)
}

func (crud *CRUD[T]) makeArray() []T {
	return make([]T, 0)
}

func (crud *CRUD[T]) handleSearches(context *gin.Context, db *gorm.DB) *gorm.DB {
	if crud.SearchHandlers != nil {
		query := context.Request.URL.Query()
		for key, value := range query {
			if handler, ok := crud.SearchHandlers[key]; ok {
				slices.Reverse(value)
				db = handler(db, value, query)
			}
		}
	}
	return db
}

func (crud *CRUD[T]) ok(context *gin.Context, data any) {
	if crud.MakeOkResponse != nil {
		crud.MakeOkResponse(context, data)
	} else {
		context.JSON(http.StatusOK, R[any]{
			Code: crud.Coder.OK(),
			Data: data,
		})
	}
}

func (crud *CRUD[T]) error(context *gin.Context, code Code, err error) {
	if crud.MakeErrorResponse != nil {
		crud.MakeErrorResponse(context, code, err)
	} else {
		MakeErrorResponse(context, code, err)
	}
}

func (crud *CRUD[T]) all(context *gin.Context) {
	db := crud.database.Model(crud.makeOne())
	db = crud.handleSearches(context, db)

	if crud.WillGetAll != nil {
		if db = crud.WillGetAll(context, db); context.IsAborted() {
			return
		}
	}

	list := crud.makeArray()
	err := db.Find(&list).Error
	if err != nil {
		crud.error(context, crud.Coder.InternalServerError(), err)
		return
	}

	if crud.DidGetAll != nil {
		if crud.DidGetAll(list, context, crud.database); context.IsAborted() {
			return
		}
	}

	crud.ok(context, list)
}

func (crud *CRUD[T]) one(context *gin.Context) {
	var result T

	id := context.Param("id")
	if id == "" {
		crud.error(context, crud.Coder.BadRequest(), errors.New("invalid ID"))
		return
	}

	if crud.WillGetOne != nil {
		if crud.WillGetOne(context, crud.database); context.IsAborted() {
			return
		}
	}

	err := crud.database.Model(crud.makeOne()).First(&result, id).Error
	if err != nil {
		crud.error(context, crud.Coder.NotFound(), err)
		return
	}

	if crud.DidGetOne != nil {
		if crud.DidGetOne(&result, context, crud.database); context.IsAborted() {
			return
		}
	}

	crud.ok(context, result)
}

func (crud *CRUD[T]) page(context *gin.Context) {
	pageNum, err := strconv.ParseInt(context.Param("pageNum"), 10, 64)
	if err != nil {
		crud.error(context, crud.Coder.BadRequest(), err)
		return
	}
	pageSize, err := strconv.ParseInt(context.Param("pageSize"), 10, 64)
	if err != nil {
		crud.error(context, crud.Coder.BadRequest(), err)
		return
	}

	if pageNum <= 0 {
		pageNum = 1
	}
	if pageSize <= 0 || (crud.DisallowAnyPageSize && !slices.Contains(crud.PageSizes, pageSize)) {
		pageSize = crud.DefaultPageSize
	}

	list := crud.makeArray()
	db := crud.database.Model(crud.makeOne())

	db = crud.handleSearches(context, db)

	if crud.WillPage != nil {
		if crud.WillPage(&pageNum, &pageSize, context, db); context.IsAborted() {
			return
		}
	}

	db.Offset(int((pageNum - 1) * pageSize))
	db.Limit(int(pageSize))
	err = db.Find(&list).Error
	if err != nil {
		crud.error(context, crud.Coder.InternalServerError(), err)
		return
	}

	if crud.DidPage != nil {
		if crud.DidPage(pageNum, pageSize, list, context, db); context.IsAborted() {
			return
		}
	}

	crud.ok(context, list)
}

func (crud *CRUD[T]) count(context *gin.Context) {
	db := crud.database.Model(crud.makeOne())
	db = crud.handleSearches(context, db)

	if crud.WillCount != nil {
		if db = crud.WillCount(context, db); context.IsAborted() {
			return
		}
	}

	var count int64
	err := db.Count(&count).Error
	if err != nil {
		crud.error(context, crud.Coder.InternalServerError(), err)
		return
	}

	if crud.DidCount != nil {
		if crud.DidCount(&count, context, db); context.IsAborted() {
			return
		}
	}

	crud.ok(context, count)
}

func (crud *CRUD[T]) save(context *gin.Context) {
	record := crud.makeOne()
	err := context.ShouldBindJSON(record)
	if err != nil {
		crud.error(context, crud.Coder.BadRequest(), err)
		return
	}

	if crud.WillSave != nil {
		if crud.WillSave(record, context, crud.database); context.IsAborted() {
			return
		}
	}

	res := crud.database.Save(record)
	if res.Error != nil {
		crud.error(context, crud.Coder.InternalServerError(), res.Error)
		return
	}

	if crud.DidSave != nil {
		if crud.DidSave(record, context, res); context.IsAborted() {
			return
		}
	}

	crud.ok(context, Ternary[any](
		res.RowsAffected > 0,
		record,
		false,
	))
}

func (crud *CRUD[T]) delete(context *gin.Context) {
	deleted := false

	if crud.WillDelete != nil {
		if crud.WillDelete(context, crud.database); context.IsAborted() {
			return
		}
	}

	if deleted = crud.OnDelete(context, crud.database); context.IsAborted() {
		return
	}

	if crud.DidDelete != nil {
		if crud.DidDelete(context, crud.database); context.IsAborted() {
			return
		}
	}

	crud.ok(context, deleted)
}

func New[T any](group *gin.RouterGroup, database *gorm.DB, crud CRUD[T]) error {
	if group == nil {
		return NilGroupError
	}
	if database == nil {
		return NilRepositoryError
	}

	crud.group = group
	crud.database = database

	if crud.Coder == nil {
		crud.Coder = RestCoder
	}

	crud.DefaultPageSize = Ternary(
		crud.DefaultPageSize <= 0,
		DefaultPageSize,
		crud.DefaultPageSize,
	)
	crud.PageSizes = Ternary(
		len(crud.PageSizes) > 0,
		crud.PageSizes,
		DefaultPageSizes,
	)

	if crud.OnDelete == nil {
		crud.OnDelete = NewHardDeleteHandler[T](crud.Coder)
	}

	if !crud.DisablePage {
		crud.group.GET("/page/:pageNum/:pageSize", crud.page)
	}

	if crud.EnableGetAll {
		crud.group.GET("/all", crud.all)
	}

	if !crud.DisableCount {
		crud.group.GET("/count", crud.count)
	}

	if !crud.DisableGetOne {
		crud.group.GET("/one/:id", crud.one)
	}

	if !crud.DisableSave {
		crud.group.PUT("", crud.save)
	}

	if !crud.DisableDelete {
		crud.group.DELETE("/:id", crud.delete)
	}

	return nil
}
