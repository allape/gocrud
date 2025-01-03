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

	WillSave func(record *T, context *gin.Context)
	DidSave  func(record *T, context *gin.Context, result *gorm.DB, db *gorm.DB)

	WillDelete func(context *gin.Context, db *gorm.DB)
	OnDelete   func(context *gin.Context, db *gorm.DB) bool
	DidDelete  func(context *gin.Context, db *gorm.DB)

	Coder             Coder
	MakeOkResponse    func(context *gin.Context, data any)
	MakeErrorResponse func(context *gin.Context, code Code, err error)

	group    *gin.RouterGroup
	database *gorm.DB
}

func (crudy *CRUD[T]) makeOne() *T {
	return new(T)
}

func (crudy *CRUD[T]) makeArray() []T {
	return make([]T, 0)
}

func (crudy *CRUD[T]) handleSearches(context *gin.Context, db *gorm.DB) *gorm.DB {
	if crudy.SearchHandlers != nil {
		with := context.Request.URL.Query()
		for key, value := range with {
			if handler, ok := crudy.SearchHandlers[key]; ok {
				db = handler(db, value, with)
			}
		}
	}
	return db
}

func (crudy *CRUD[T]) ok(context *gin.Context, data any) {
	if crudy.MakeOkResponse != nil {
		crudy.MakeOkResponse(context, data)
	} else {
		context.JSON(http.StatusOK, R[any]{
			Code: crudy.Coder.OK(),
			Data: data,
		})
	}
}

func (crudy *CRUD[T]) error(context *gin.Context, code Code, err error) {
	if crudy.MakeErrorResponse != nil {
		crudy.MakeErrorResponse(context, code, err)
	} else {
		MakeErrorResponse(context, code, err)
	}
}

func (crudy *CRUD[T]) all(context *gin.Context) {
	db := crudy.database.Model(crudy.makeOne())
	db = crudy.handleSearches(context, db)

	if crudy.WillGetAll != nil {
		if db = crudy.WillGetAll(context, db); context.IsAborted() {
			return
		}
	}

	list := crudy.makeArray()
	err := db.Find(&list).Error
	if err != nil {
		crudy.error(context, crudy.Coder.InternalServerError(), err)
		return
	}

	if crudy.DidGetAll != nil {
		if crudy.DidGetAll(list, context, crudy.database); context.IsAborted() {
			return
		}
	}

	crudy.ok(context, list)
}

func (crudy *CRUD[T]) one(context *gin.Context) {
	var result T

	id := context.Param("id")
	if id == "" {
		crudy.error(context, crudy.Coder.BadRequest(), errors.New("invalid ID"))
		return
	}

	if crudy.WillGetOne != nil {
		if crudy.WillGetOne(context, crudy.database); context.IsAborted() {
			return
		}
	}

	err := crudy.database.Model(crudy.makeOne()).First(&result, id).Error
	if err != nil {
		crudy.error(context, crudy.Coder.NotFound(), err)
		return
	}

	if crudy.DidGetOne != nil {
		if crudy.DidGetOne(&result, context, crudy.database); context.IsAborted() {
			return
		}
	}

	crudy.ok(context, result)
}

func (crudy *CRUD[T]) page(context *gin.Context) {
	pageNum, err := strconv.ParseInt(context.Param("pageNum"), 10, 64)
	if err != nil {
		crudy.error(context, crudy.Coder.BadRequest(), err)
		return
	}
	pageSize, err := strconv.ParseInt(context.Param("pageSize"), 10, 64)
	if err != nil {
		crudy.error(context, crudy.Coder.BadRequest(), err)
		return
	}

	if pageNum <= 0 {
		pageNum = 1
	}
	if pageSize <= 0 || (crudy.DisallowAnyPageSize && !slices.Contains(crudy.PageSizes, pageSize)) {
		pageSize = crudy.DefaultPageSize
	}

	list := crudy.makeArray()
	db := crudy.database.Model(crudy.makeOne())

	db = crudy.handleSearches(context, db)

	if crudy.WillPage != nil {
		if crudy.WillPage(&pageNum, &pageSize, context, db); context.IsAborted() {
			return
		}
	}

	db.Offset(int((pageNum - 1) * pageSize))
	db.Limit(int(pageSize))
	err = db.Find(&list).Error
	if err != nil {
		crudy.error(context, crudy.Coder.InternalServerError(), err)
		return
	}

	if crudy.DidPage != nil {
		if crudy.DidPage(pageNum, pageSize, list, context, db); context.IsAborted() {
			return
		}
	}

	crudy.ok(context, list)
}

func (crudy *CRUD[T]) count(context *gin.Context) {
	db := crudy.database.Model(crudy.makeOne())
	db = crudy.handleSearches(context, db)

	if crudy.WillCount != nil {
		if db = crudy.WillCount(context, db); context.IsAborted() {
			return
		}
	}

	var count int64
	err := db.Count(&count).Error
	if err != nil {
		crudy.error(context, crudy.Coder.InternalServerError(), err)
		return
	}

	if crudy.DidCount != nil {
		if crudy.DidCount(&count, context, db); context.IsAborted() {
			return
		}
	}

	crudy.ok(context, count)
}

func (crudy *CRUD[T]) save(context *gin.Context) {
	record := crudy.makeOne()
	err := context.ShouldBindJSON(record)
	if err != nil {
		crudy.error(context, crudy.Coder.BadRequest(), err)
		return
	}

	if crudy.WillSave != nil {
		if crudy.WillSave(record, context); context.IsAborted() {
			return
		}
	}

	res := crudy.database.Save(record)
	if res.Error != nil {
		crudy.error(context, crudy.Coder.InternalServerError(), res.Error)
		return
	}

	if crudy.DidSave != nil {
		if crudy.DidSave(record, context, res, crudy.database); context.IsAborted() {
			return
		}
	}

	crudy.ok(context, Ternary[any](
		res.RowsAffected > 0,
		record,
		false,
	))
}

func (crudy *CRUD[T]) delete(context *gin.Context) {
	deleted := false

	if crudy.WillDelete != nil {
		if crudy.WillDelete(context, crudy.database); context.IsAborted() {
			return
		}
	}

	if deleted = crudy.OnDelete(context, crudy.database); context.IsAborted() {
		return
	}

	if crudy.DidDelete != nil {
		if crudy.DidDelete(context, crudy.database); context.IsAborted() {
			return
		}
	}

	crudy.ok(context, deleted)
}

func New[T any](group *gin.RouterGroup, database *gorm.DB, crudy CRUD[T]) error {
	if group == nil {
		return NilGroupError
	}
	if database == nil {
		return NilRepositoryError
	}

	crudy.group = group
	crudy.database = database

	if crudy.Coder == nil {
		crudy.Coder = RestCoder
	}

	crudy.DefaultPageSize = Ternary(
		crudy.DefaultPageSize <= 0,
		DefaultPageSize,
		crudy.DefaultPageSize,
	)
	crudy.PageSizes = Ternary(
		len(crudy.PageSizes) > 0,
		crudy.PageSizes,
		DefaultPageSizes,
	)

	if crudy.OnDelete == nil {
		crudy.OnDelete = NewHardDeleteHandler[T](crudy.Coder)
	}

	if !crudy.DisablePage {
		crudy.group.GET("/page/:pageNum/:pageSize", crudy.page)
	}

	if crudy.EnableGetAll {
		crudy.group.GET("/all", crudy.all)
	}

	if !crudy.DisableCount {
		crudy.group.GET("/count", crudy.count)
	}

	if !crudy.DisableGetOne {
		crudy.group.GET("/one/:id", crudy.one)
	}

	if !crudy.DisableSave {
		crudy.group.PUT("", crudy.save)
	}

	if !crudy.DisableDelete {
		crudy.group.DELETE("/:id", crudy.delete)
	}

	return nil
}
