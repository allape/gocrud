package gocrud

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/allape/gocensored"
	"github.com/allape/gogger"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var (
	NilGroupError    = errors.New("group is nil")
	NilDatabaseError = errors.New("database is nil")
)

var (
	DefaultPageSizes = []uint64{10, 20, 50, 100}
	DefaultPageSize  = uint64(50)
)

type Crud[T any] struct {
	DisallowNonstandardPageSize bool
	DefaultPageSize             uint64
	PageSizes                   []uint64

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

	WillPage func(pageNum *uint64, pageSize *uint64, context *gin.Context, db *gorm.DB) *gorm.DB
	DidPage  func(pageNum uint64, pageSize uint64, list []T, context *gin.Context, db *gorm.DB)

	WillSave func(record *T, context *gin.Context, db *gorm.DB)
	DidSave  func(record *T, context *gin.Context, db *gorm.DB)

	WillDelete func(context *gin.Context, db *gorm.DB)
	OnDelete   func(context *gin.Context, db *gorm.DB) bool
	DidDelete  func(context *gin.Context, db *gorm.DB)

	Coder             Coder
	MakeOkayResponse  func(context *gin.Context, data any)
	MakeErrorResponse func(context *gin.Context, code Code, err any)

	GetCensors func(context *gin.Context, db *gorm.DB) ([]*censored.Censor, error)

	group    *gin.RouterGroup
	database *gorm.DB
	logger   *gogger.Logger
}

// region censors

func (crud *Crud[T]) encensor(context *gin.Context, db *gorm.DB, record *T) error {
	return crud.docensor(context, db, record, true)
}

func (crud *Crud[T]) decensor(context *gin.Context, db *gorm.DB, record *T) error {
	return crud.docensor(context, db, record, false)
}

func (crud *Crud[T]) docensor(context *gin.Context, db *gorm.DB, record *T, encensor bool) error {
	censors, err := crud.GetCensors(context, db)
	if err != nil {
		return err
	}
	if encensor {
		for i := 0; i < len(censors); i++ {
			err = censors[i].Encencor(record)
			if err != nil {
				return err
			}
		}
	} else {
		for i := 0; i < len(censors); i++ {
			err = censors[i].Decensor(record)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (crud *Crud[T]) decensorList(context *gin.Context, db *gorm.DB, list []T) error {
	for i := 0; i < len(list); i++ {
		err := crud.decensor(context, db, &list[i])
		if err != nil {

			return err
		}
	}
	return nil
}

// endregion

// region helper

func (crud *Crud[T]) makeOne() *T {
	return new(T)
}

func (crud *Crud[T]) makeArray() []T {
	return make([]T, 0)
}

func (crud *Crud[T]) handleSearches(context *gin.Context, db *gorm.DB) (*gorm.DB, error) {
	return HandleSearch(context, db, crud.SearchHandlers)
}

func (crud *Crud[T]) ok(context *gin.Context, data any) {
	if crud.MakeOkayResponse != nil {
		crud.MakeOkayResponse(context, data)
	} else {
		MakeOkayDataResponse(context, data)
	}
}

func (crud *Crud[T]) error(context *gin.Context, code Code, err any) {
	if crud.MakeErrorResponse != nil {
		crud.MakeErrorResponse(context, code, err)
	} else {
		MakeErrorResponse(context, code, err)
	}
}

// endregion

// region primary functions

func (crud *Crud[T]) all(context *gin.Context) {
	db := crud.database.Model(crud.makeOne())

	db, err := crud.handleSearches(context, db)
	if err != nil {
		crud.logger.Error().Printf("all: failed to handle searches: %v", err)
		crud.error(context, crud.Coder.InternalServerError(), "[error] search failed")
		return
	}

	if crud.WillGetAll != nil {
		if db = crud.WillGetAll(context, db); context.IsAborted() {
			return
		}
	}

	list := crud.makeArray()
	err = db.Find(&list).Error
	if err != nil {
		crud.logger.Error().Printf("all: failed to find records: %v", err)
		crud.error(context, crud.Coder.InternalServerError(), "[error] database failed")
		return
	}

	err = crud.decensorList(context, db, list)
	if err != nil {
		crud.logger.Error().Printf("all: failed to decensor records: %v", err)
		crud.error(context, crud.Coder.InternalServerError(), "[error] decensor failed]")
		return
	}

	if crud.DidGetAll != nil {
		if crud.DidGetAll(list, context, crud.database); context.IsAborted() {
			return
		}
	}

	crud.ok(context, list)
}

func (crud *Crud[T]) one(context *gin.Context) {
	var result T

	id := Pick(IDsFromCommaSeparatedString(context.Param("id")), 0, 0)
	if id == 0 {
		crud.error(context, crud.Coder.BadRequest(), "invalid id")
		return
	}

	if crud.WillGetOne != nil {
		if crud.WillGetOne(context, crud.database); context.IsAborted() {
			return
		}
	}

	db := crud.database.Model(crud.makeOne())

	err := db.Where("id = ?", id).First(&result).Error
	if err != nil {
		crud.logger.Error().Printf("one: failed to find record: %v", err)
		crud.error(context, crud.Coder.NotFound(), "not found")
		return
	}

	err = crud.decensor(context, db, &result)
	if err != nil {
		crud.logger.Error().Printf("one: failed to decensor record: %v", err)
		crud.error(context, crud.Coder.InternalServerError(), "[error] decensor failed")
		return
	}

	if crud.DidGetOne != nil {
		if crud.DidGetOne(&result, context, crud.database); context.IsAborted() {
			return
		}
	}

	crud.ok(context, result)
}

func (crud *Crud[T]) page(context *gin.Context) {
	pageNum, err := strconv.ParseUint(context.Param("pageNum"), 10, 64)
	if err != nil {
		crud.error(context, crud.Coder.BadRequest(), "invalid page number")
		return
	}
	pageSize, err := strconv.ParseUint(context.Param("pageSize"), 10, 64)
	if err != nil {
		crud.error(context, crud.Coder.BadRequest(), "invalid page size")
		return
	}

	if pageNum <= 0 {
		pageNum = 1
	}
	if pageSize <= 0 || (crud.DisallowNonstandardPageSize && !slices.Contains(crud.PageSizes, pageSize)) {
		pageSize = crud.DefaultPageSize
	}

	list := crud.makeArray()
	db := crud.database.Model(crud.makeOne())

	db, err = crud.handleSearches(context, db)
	if err != nil {
		crud.logger.Error().Printf("page: failed to handle searches: %v", err)
		crud.error(context, crud.Coder.InternalServerError(), "[error] search failed")
		return
	}

	if crud.WillPage != nil {
		if crud.WillPage(&pageNum, &pageSize, context, db); context.IsAborted() {
			return
		}
	}

	db = db.Offset(int((pageNum - 1) * pageSize)).Limit(int(pageSize))
	err = db.Find(&list).Error
	if err != nil {
		crud.logger.Error().Printf("page: failed to find records: %v", err)
		crud.error(context, crud.Coder.InternalServerError(), "[error] database failed")
		return
	}

	err = crud.decensorList(context, db, list)
	if err != nil {
		crud.logger.Error().Printf("page: failed to decensor records: %v", err)
		crud.error(context, crud.Coder.InternalServerError(), "[error] decensor failed")
		return
	}

	if crud.DidPage != nil {
		if crud.DidPage(pageNum, pageSize, list, context, db); context.IsAborted() {
			return
		}
	}

	crud.ok(context, list)
}

func (crud *Crud[T]) count(context *gin.Context) {
	db := crud.database.Model(crud.makeOne())
	db, err := crud.handleSearches(context, db)
	if err != nil {
		crud.logger.Error().Printf("count: failed to handle searches: %v", err)
		crud.error(context, crud.Coder.InternalServerError(), "[error] search failed")
		return
	}

	if crud.WillCount != nil {
		if db = crud.WillCount(context, db); context.IsAborted() {
			return
		}
	}

	var count int64
	err = db.Count(&count).Error
	if err != nil {
		crud.logger.Error().Printf("count: failed to count records: %v", err)
		crud.error(context, crud.Coder.InternalServerError(), "[error] count failed")
		return
	}

	if crud.DidCount != nil {
		if crud.DidCount(&count, context, db); context.IsAborted() {
			return
		}
	}

	crud.ok(context, count)
}

func (crud *Crud[T]) save(context *gin.Context) {
	record := crud.makeOne()
	err := context.ShouldBindJSON(record)
	if err != nil {
		crud.error(context, crud.Coder.BadRequest(), "invalid body")
		return
	}

	if crud.WillSave != nil {
		if crud.WillSave(record, context, crud.database); context.IsAborted() {
			return
		}
	}

	err = crud.encensor(context, crud.database, record)
	if err != nil {
		crud.logger.Error().Printf("save: failed to encensor record: %v", err)
		crud.error(context, crud.Coder.InternalServerError(), "[error] encensor failed")
		return
	}

	res := crud.database.Save(record)
	if res.Error != nil {
		crud.logger.Error().Printf("save: failed to save record: %v", res.Error)
		crud.error(context, crud.Coder.InternalServerError(), "[error] save failed")
		return
	}

	err = crud.decensor(context, crud.database, record)
	if err != nil {
		crud.logger.Error().Printf("save: failed to decensor record: %v", err)
		crud.error(context, crud.Coder.InternalServerError(), "[error] decensor failed")
		return
	}

	if crud.DidSave != nil {
		if crud.DidSave(record, context, res); context.IsAborted() {
			return
		}
	}

	crud.ok(context, record)
}

func (crud *Crud[T]) delete(context *gin.Context) {
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

// endregion

func Setup[T any](
	group *gin.RouterGroup,
	database *gorm.DB,
	logger *gogger.Logger,
	crud *Crud[T],
) error {
	if group == nil {
		return NilGroupError
	}
	if database == nil {
		return NilDatabaseError
	}

	if crud == nil {
		crud = &Crud[T]{}
	}

	crud.group = group
	crud.database = database
	crud.logger = logger

	if crud.logger == nil {
		name := strings.ToLower(reflect.TypeOf(crud.makeOne()).Elem().Name())
		crud.logger = gogger.New(fmt.Sprintf("crud:%s", name))
	}

	if crud.Coder == nil {
		crud.Coder = RestCoder
	}

	if crud.GetCensors == nil {
		crud.GetCensors = func(context *gin.Context, db *gorm.DB) ([]*censored.Censor, error) {
			return nil, nil
		}
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
		crud.OnDelete = NewSoftDeleteHandler[T](crud.Coder)
	}

	if !crud.DisablePage {
		crud.group.GET("/page/:pageNum/:pageSize", crud.page)
		crud.group.POST("/page/:pageNum/:pageSize", crud.page)
	}

	if crud.EnableGetAll {
		crud.group.GET("/all", crud.all)
		crud.group.POST("/all", crud.all)
	}

	if !crud.DisableCount {
		crud.group.GET("/count", crud.count)
		crud.group.POST("/count", crud.count)
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
