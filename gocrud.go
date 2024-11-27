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

var NilGroupError = errors.New("engine is nil")
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

	WillGetAll func(ctx *gin.Context, repo *gorm.DB) error
	DidGetAll  func(record []T, ctx *gin.Context, repo *gorm.DB) error

	OnGetOne  func(ctx *gin.Context, repo *gorm.DB) (T, error)
	DidGetOne func(record *T, ctx *gin.Context, repo *gorm.DB) error

	WillPage func(pageNum *int64, pageSize *int64, ctx *gin.Context) error
	DidPage  func(pageNum int64, pageSize int64, list []T, ctx *gin.Context) error

	WillSave func(record *T, ctx *gin.Context) error
	DidSave  func(record *T, ctx *gin.Context, result *gorm.DB, repo *gorm.DB) error

	OnDelete  func(ctx *gin.Context, repo *gorm.DB) (bool, error)
	DidDelete func(ctx *gin.Context, repo *gorm.DB) error

	Coder             Coder
	MakeOkResponse    func(ctx *gin.Context, data any)
	MakeErrorResponse func(ctx *gin.Context, code Code, err error)

	group      *gin.RouterGroup
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
			Code: c.Coder.OK(),
			Data: data,
		})
	}
}

func (c *CRUD[T]) error(context *gin.Context, code Code, err error) {
	if c.MakeErrorResponse != nil {
		c.MakeErrorResponse(context, code, err)
	} else {
		MakeErrorResponse(context, code, err)
	}
}

func (c *CRUD[T]) all(context *gin.Context) {
	repo := c.repository.Model(c.makeOne())
	if c.WillGetAll != nil {
		err := c.WillGetAll(context, c.repository)
		if err != nil {
			c.error(context, c.Coder.InternalServerError(), err)
			return
		}
	}
	c.handleSearches(context, repo)
	list := c.makeArray()
	repo.Find(&list)
	if c.DidGetAll != nil {
		err := c.DidGetAll(list, context, c.repository)
		if err != nil {
			c.error(context, c.Coder.InternalServerError(), err)
			return
		}
	}
	c.ok(context, list)
}

func (c *CRUD[T]) one(context *gin.Context) {
	var err error
	var result T
	if c.OnGetOne != nil {
		result, err = c.OnGetOne(context, c.repository)
		if err != nil {
			c.error(context, c.Coder.InternalServerError(), err)
			return
		}
	} else {
		id := context.Query("id")
		if id == "" {
			c.error(context, c.Coder.BadRequest(), errors.New("invalid ID"))
			return
		}
		res := c.repository.Model(c.makeOne()).Where("id = ?", id).First(&result)
		if res.RowsAffected == 0 {
			if res.Error == nil {
				c.error(context, c.Coder.NotFound(), errors.New("record not found"))
				return
			} else {
				c.error(context, c.Coder.InternalServerError(), res.Error)
				return
			}
		}
	}
	if c.DidGetOne != nil {
		err := c.DidGetOne(&result, context, c.repository)
		if err != nil {
			c.error(context, c.Coder.InternalServerError(), err)
			return
		}
	}
	c.ok(context, result)
}

func (c *CRUD[T]) page(context *gin.Context) {
	pageNum, err := strconv.ParseInt(context.Param("pageNum"), 10, 64)
	if err != nil {
		c.error(context, c.Coder.BadRequest(), err)
		return
	}
	pageSize, err := strconv.ParseInt(context.Param("pageSize"), 10, 64)
	if err != nil {
		c.error(context, c.Coder.BadRequest(), err)
		return
	}

	if pageNum <= 0 {
		pageNum = 1
	}
	if pageSize <= 0 || (!c.AllowAnyPageSize && !slices.Contains(c.PageSizes, pageSize)) {
		pageSize = c.DefaultPageSize
	}

	if c.WillPage != nil {
		err := c.WillPage(&pageNum, &pageSize, context)
		if err != nil {
			c.error(context, c.Coder.InternalServerError(), err)
			return
		}
	}

	list := c.makeArray()
	repo := c.repository.Model(c.makeOne())

	c.handleSearches(context, repo)

	repo.Offset(int((pageNum - 1) * pageSize))
	repo.Limit(int(pageSize))
	repo.Find(&list)

	if c.DidPage != nil {
		err := c.DidPage(pageNum, pageSize, list, context)
		if err != nil {
			c.error(context, c.Coder.InternalServerError(), err)
			return
		}
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
		c.error(context, c.Coder.BadRequest(), err)
		return
	}

	if c.WillSave != nil {
		err := c.WillSave(record, context)
		if err != nil {
			c.error(context, c.Coder.InternalServerError(), err)
			return
		}
	}

	res := c.repository.Save(record)
	if res.Error != nil {
		c.error(context, c.Coder.InternalServerError(), res.Error)
		return
	}

	if c.DidSave != nil {
		err := c.DidSave(record, context, res, c.repository)
		if err != nil {
			c.error(context, c.Coder.InternalServerError(), err)
			return
		}
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
			c.error(context, c.Coder.BadRequest(), errors.New("invalid ID"))
			return
		}

		repo := c.repository.Delete(c.makeOne(), id)
		err = repo.Error
		deleted = repo.RowsAffected > 0
	}

	if c.DidDelete != nil {
		err := c.DidDelete(context, c.repository)
		if err != nil {
			c.error(context, c.Coder.InternalServerError(), err)
			return
		}
	}

	if err != nil {
		c.error(context, c.Coder.InternalServerError(), err)
		return
	}

	c.ok(context, deleted)
}

func (c *CRUD[T]) Setup(group *gin.RouterGroup, repository *gorm.DB) error {
	if group == nil {
		return NilGroupError
	}
	if repository == nil {
		return NilRepositoryError
	}

	c.group = group
	c.repository = repository

	if c.Coder == nil {
		c.Coder = NewDefaultCoder()
	}

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
		c.group.GET("", c.one)
	}

	if c.EnableGetAll {
		c.group.GET("/all", c.all)
	}

	if !c.DisablePage {
		c.group.GET("/:pageNum/:pageSize", c.page)
	}

	if !c.DisableCount {
		c.group.GET("/count", c.count)
	}

	if !c.DisableSave {
		c.group.PUT("", c.save)
	}

	if !c.DisableDelete {
		c.group.DELETE("", c.delete)
	}

	return nil
}
