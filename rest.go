package gocrud

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http"
	"net/url"
	"strings"
)

var RestCoder = NewDefaultCoder()

type R[T any] struct {
	Code    Code   `json:"c"`
	Message string `json:"m"`
	Data    T      `json:"d"`
}

func MakeErrorResponse(context *gin.Context, code Code, err any) {
	message := "Internal Server Error"

	if err != nil {
		switch err.(type) {
		case string:
			message = ValuableString(Pointer(err.(string)), message)
		case *string:
			message = ValuableString(err.(*string), message)
		case error:
			message = ValuableString(Pointer(err.(error).Error()), message)
		}
	}

	context.AbortWithStatusJSON(http.StatusOK, R[any]{
		Code:    Ternary(code == "", RestCoder.InternalServerError(), code),
		Message: message,
		Data:    err,
	})
}

func RecoveryHandler(responseFullError bool) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, err any) {
		MakeErrorResponse(
			c,
			RestCoder.InternalServerError(),
			Ternary(responseFullError, err, nil),
		)
	})
}

// SortBy
// `"id": SortBy("id")` with `?id=asc`                        -> SELECT ... FROM ... ORDER BY `id` ASC
// `"createdAt": SortBy("created_at")` with `?createdAt=desc` -> SELECT ... FROM ... ORDER BY `created_at` DESC
func SortBy(name string) SearchHandler {
	return func(db *gorm.DB, values []string, _ url.Values) {
		if ok, value := ValuableStringFromArray(values); ok {
			sort := "asc"
			if strings.ToLower(value) == "desc" {
				sort = "desc"
			}
			db.Order(fmt.Sprintf("`%s` %s", name, sort))
		}
	}
}

// KeywordIn
// `"id": KeywordIn("id", nil)` with `?id=1,2,3` -> SELECT ... FROM ... WHERE `id` in (1, 2, 3)
func KeywordIn(name string, filterFunc func(value []string) []string) SearchHandler {
	return func(db *gorm.DB, values []string, _ url.Values) {
		if ok, value := ValuableStringFromArray(values); ok {
			values = strings.Split(value, ",")

			var parsedValues []string
			for _, s := range values {
				s = strings.TrimSpace(s)
				if s != "" {
					parsedValues = append(parsedValues, s)
				}
			}

			if filterFunc != nil {
				parsedValues = filterFunc(parsedValues)
				if len(parsedValues) == 0 {
					return
				}
			}

			db.Where(
				fmt.Sprintf("`%s` in ?", name),
				parsedValues,
			)
		}
	}
}

// KeywordLike
// `"name": KeywordLike("name")` with `?name=foo` -> SELECT ... FROM ... WHERE `name` like '%foo%'
func KeywordLike(name string, filterFunc func(value string) string) SearchHandler {
	return func(db *gorm.DB, values []string, _ url.Values) {
		if ok, value := ValuableStringFromArray(values); ok {
			if filterFunc != nil {
				value = filterFunc(value)
			}
			db.Where(
				fmt.Sprintf("`%s` like ?", name),
				fmt.Sprintf("%%%s%%", value),
			)
		}
	}
}

// KeywordEqual
// `"id": KeywordEqual("id")` with `?id=1` -> SELECT ... FROM ... WHERE `id` = 1
func KeywordEqual(name string, filterFunc func(value string) any) SearchHandler {
	return func(db *gorm.DB, values []string, _ url.Values) {
		if ok, value := ValuableStringFromArray(values); ok {
			var anyValue any = value
			if filterFunc != nil {
				anyValue = filterFunc(value)
			}
			db.Where(
				fmt.Sprintf("`%s` = ?", name),
				anyValue,
			)
		}
	}
}
