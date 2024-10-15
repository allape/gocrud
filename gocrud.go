package gocrud

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http"
	"net/url"
	"slices"
	"strconv"
)

var NilEngineError = errors.New("engine is nil")
var NilRepositoryError = errors.New("repository is nil")

var (
	DefaultPageSizes = []int64{10, 20, 50, 100}
	DefaultPageSize  = DefaultPageSizes[0]
)

type (
	SearchHandler  = func(db *gorm.DB, values []string, with url.Values)
	SearchHandlers = map[string]SearchHandler
)

type CRUD[T any] struct {
	AllowAnyPageSize bool
	DefaultPageSize  int64
	PageSizes        []int64

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

	WillGetAll func(ctx *gin.Context, repo *gorm.DB)
	DidGetAll  func(record []T, ctx *gin.Context, repo *gorm.DB)

	OnGetOne  func(ctx *gin.Context, repo *gorm.DB) (T, error)
	DidGetOne func(record *T, ctx *gin.Context, repo *gorm.DB)

	WillPage func(pageNum *int64, pageSize *int64, ctx *gin.Context)
	DidPage  func(pageNum int64, pageSize int64, list []T, ctx *gin.Context)

	WillSave func(record *T, ctx *gin.Context)
	DidSave  func(record *T, ctx *gin.Context, result *gorm.DB, repo *gorm.DB)

	OnDelete  func(ctx *gin.Context, repo *gorm.DB) (bool, error)
	DidDelete func(ctx *gin.Context, repo *gorm.DB)

	MakeOkResponse    func(ctx *gin.Context, data any)
	MakeErrorResponse func(ctx *gin.Context, suggestedHttpStatusCode int, err error)

	prefix     string
	router     gin.IRouter
	repository *gorm.DB
}

func (c *CRUD[T]) makeOne() *T {
	return new(T)
}

func (c *CRUD[T]) makeArray() []T {
	return make([]T, 0)
}

func (c *CRUD[T]) handleSearches(context *gin.Context, repo *gorm.DB) {
	if c.SearchHandlers != nil {
		with := context.Request.URL.Query()
		for key, value := range with {
			if handler, ok := c.SearchHandlers[key]; ok {
				handler(repo, value, with)
			}
		}
	}
}

func (c *CRUD[T]) ok(context *gin.Context, data any) {
	if c.MakeOkResponse != nil {
		c.MakeOkResponse(context, data)
	} else {
		context.JSON(http.StatusOK, R[any]{
			Code:    "0",
			Message: "OK",
			Data:    data,
		})
	}
}

func (c *CRUD[T]) error(context *gin.Context, suggestedHttpStatusCode int, err error) {
	if c.MakeErrorResponse != nil {
		c.MakeErrorResponse(context, suggestedHttpStatusCode, err)
	} else {
		context.JSON(suggestedHttpStatusCode, R[any]{
			Code:    fmt.Sprintf("%d", suggestedHttpStatusCode),
			Message: err.Error(),
		})
	}
}

func (c *CRUD[T]) all(context *gin.Context) {
	repo := c.repository.Model(c.makeOne())
	if c.WillGetAll != nil {
		c.WillGetAll(context, c.repository)
	}
	c.handleSearches(context, repo)
	list := c.makeArray()
	repo.Find(&list)
	if c.DidGetAll != nil {
		c.DidGetAll(list, context, c.repository)
	}
	c.ok(context, list)
}

func (c *CRUD[T]) one(context *gin.Context) {
	var err error
	var result T
	if c.OnGetOne != nil {
		result, err = c.OnGetOne(context, c.repository)
		if err != nil {
			c.error(context, http.StatusInternalServerError, err)
			return
		}
	} else {
		id := context.Query("id")
		if id == "" {
			c.error(context, http.StatusBadRequest, errors.New("invalid ID"))
			return
		}
		res := c.repository.Model(c.makeOne()).Where("id = ?", id).First(&result)
		if res.RowsAffected == 0 {
			if res.Error == nil {
				c.error(context, http.StatusNotFound, errors.New("record not found"))
				return
			} else {
				c.error(context, http.StatusInternalServerError, res.Error)
				return
			}
		}
	}
	if c.DidGetOne != nil {
		c.DidGetOne(&result, context, c.repository)
	}
	c.ok(context, result)
}

func (c *CRUD[T]) page(context *gin.Context) {
	pageNum, err := strconv.ParseInt(context.Param("pageNum"), 10, 64)
	if err != nil {
		c.error(context, http.StatusBadRequest, err)
		return
	}
	pageSize, err := strconv.ParseInt(context.Param("pageSize"), 10, 64)
	if err != nil {
		c.error(context, http.StatusBadRequest, err)
		return
	}

	if pageNum <= 0 {
		pageNum = 1
	}
	if pageSize <= 0 || (!c.AllowAnyPageSize && !slices.Contains(c.PageSizes, pageSize)) {
		pageSize = c.DefaultPageSize
	}

	if c.WillPage != nil {
		c.WillPage(&pageNum, &pageSize, context)
	}

	list := c.makeArray()
	repo := c.repository.Model(c.makeOne())

	c.handleSearches(context, repo)

	repo.Offset(int((pageNum - 1) * pageSize))
	repo.Limit(int(pageSize))
	repo.Find(&list)

	if c.DidPage != nil {
		c.DidPage(pageNum, pageSize, list, context)
	}

	c.ok(context, list)
}

func (c *CRUD[T]) count(context *gin.Context) {
	repo := c.repository.Model(c.makeOne())
	c.handleSearches(context, repo)

	var count int64
	repo.Count(&count)

	c.ok(context, count)
}

func (c *CRUD[T]) save(context *gin.Context) {
	record := c.makeOne()
	err := context.ShouldBindJSON(record)
	if err != nil {
		c.error(context, http.StatusBadRequest, err)
		return
	}

	if c.WillSave != nil {
		c.WillSave(record, context)
	}

	res := c.repository.Save(record)
	if res.Error != nil {
		c.error(context, http.StatusInternalServerError, res.Error)
		return
	}

	if c.DidSave != nil {
		c.DidSave(record, context, res, c.repository)
	}

	c.ok(context, Ternary[any](
		res.RowsAffected > 0,
		record,
		false,
	))
}

func (c *CRUD[T]) delete(context *gin.Context) {
	var deleted = false
	var err error

	if c.OnDelete != nil {
		deleted, err = c.OnDelete(context, c.repository)
	} else {
		id := context.Query("id")

		if id == "" {
			c.error(context, http.StatusBadRequest, errors.New("invalid ID"))
			return
		}

		repo := c.repository.Delete(c.makeOne(), id)
		err = repo.Error
		deleted = repo.RowsAffected > 0
	}

	if c.DidDelete != nil {
		c.DidDelete(context, c.repository)
	}

	if err != nil {
		c.error(context, http.StatusInternalServerError, err)
		return
	}

	c.ok(context, deleted)
}

func (c *CRUD[T]) Setup(prefix string, router gin.IRouter, repository *gorm.DB) error {
	if router == nil {
		return NilEngineError
	}
	if repository == nil {
		return NilRepositoryError
	}

	c.router = router
	c.prefix = prefix
	c.repository = repository

	c.DefaultPageSize = Ternary(
		c.DefaultPageSize <= 0,
		DefaultPageSize,
		c.DefaultPageSize,
	)
	c.PageSizes = Ternary(
		len(c.PageSizes) > 0,
		c.PageSizes,
		DefaultPageSizes,
	)

	if !c.DisableGetOne {
		c.router.GET(c.prefix, c.one)
	}

	if c.EnableGetAll {
		c.router.GET(c.prefix+"/all", c.all)
	}

	if !c.DisablePage {
		c.router.GET(c.prefix+"/:pageNum/:pageSize", c.page)
	}

	if !c.DisableCount {
		c.router.GET(c.prefix+"/count", c.count)
	}

	if !c.DisableSave {
		c.router.PUT(c.prefix, c.save)
	}

	if !c.DisableDelete {
		c.router.DELETE(c.prefix, c.delete)
	}

	return nil
}
