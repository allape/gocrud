package gocrud

import (
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type (
	SearchHandler  = func(db *gorm.DB, values []string, context *gin.Context) (*gorm.DB, error)
	SearchHandlers = map[string]SearchHandler
)

const ContextKeyHandledSearch = "gocrud:crud:hanldedsearch"

func GetHandledSearch(context *gin.Context) []string {
	return context.GetStringSlice(ContextKeyHandledSearch)
}

func SetHandledSearch(context *gin.Context, handledSearch []string) {
	context.Set(ContextKeyHandledSearch, handledSearch)
}

func HandleSearch(context *gin.Context, db *gorm.DB, searchHandlers SearchHandlers) (*gorm.DB, error) {
	if searchHandlers == nil {
		return db, nil
	}

	searches, err := GetSearchValuesFromContext(context)
	if err != nil {
		return nil, err
	}

	handledSearch := make([]string, 0, len(searches))

	for key, value := range searches {
		if handler, ok := searchHandlers[key]; ok {
			db, err = handler(db, value, context)
			if err != nil {
				return nil, err
			}
			handledSearch = append(handledSearch, key)
		}
	}

	SetHandledSearch(context, handledSearch)

	return db, nil
}

func MergeSearchHandlers(searchHandlers SearchHandlers, extraSearchHandlers ...SearchHandlers) SearchHandlers {
	return MergeMaps(searchHandlers, extraSearchHandlers...)
}

type (
	Operator                       string
	ValueTransformer[T any, R any] func(value T) R
	AnyValueTransformer            ValueTransformer[string, any]
	StringValueTransformer         ValueTransformer[string, string]
)

var (
	NotArrayError = fmt.Errorf("not an array")
)

const (
	OperatorEqual      Operator = "="
	OperatorLike       Operator = "LIKE"
	OperatorNotLike    Operator = "NOT LIKE"
	OperatorIn         Operator = "IN"
	OperatorNotIn      Operator = "NOT IN"
	OperatorNull       Operator = "IS NULL"
	OperatorNNull      Operator = "IS NOT NULL"
	OperatorBetween    Operator = "BETWEEN"
	OperatorNotBetween Operator = "NOT BETWEEN"
	OperatorGt         Operator = ">"
	OperatorGte        Operator = ">="
	OperatorLt         Operator = "<"
	OperatorLte        Operator = "<="
	OperatorNeq        Operator = "!="
)

var Operators = []Operator{
	OperatorEqual,
	OperatorLike,
	OperatorNotLike,
	OperatorIn,
	OperatorNotIn,
	OperatorNull,
	OperatorNNull,
	OperatorBetween,
	OperatorNotBetween,
	OperatorGt,
	OperatorGte,
	OperatorLt,
	OperatorLte,
	OperatorNeq,
}

func KeywordStatement(field string, operator Operator, vts ...AnyValueTransformer) SearchHandler {
	if !slices.Contains(Operators, operator) {
		panic(fmt.Sprintf("operator %s is not a valid operator", operator))
	}

	return func(db *gorm.DB, values []string, _ *gin.Context) (*gorm.DB, error) {
		if value, ok := PickFirstValuableString(values); ok {
			var anyValue any = nil

			for _, vt := range vts {
				if vt != nil {
					v := vt(value)
					if v != nil {
						anyValue = v
						break
					}
				}
			}

			if anyValue == nil {
				return db, nil
			}

			if operator == OperatorIn || operator == OperatorNotIn {
				if !IsNotEmptyArray(anyValue) {
					return nil, NotArrayError
				}
			}

			stat := TernaryFunc(func() bool {
				return strings.Contains(field, ".") || strings.Contains(field, "`")
			}, func() string {
				return fmt.Sprintf("%s %s ?", field, operator)
			}, func() string {
				return fmt.Sprintf("`%s` %s ?", field, operator)
			})

			db = db.Where(
				stat,
				anyValue,
			)
		}

		return db, nil
	}
}

func KeywordIn(field string, vts ...ValueTransformer[[]string, []string]) SearchHandler {
	return KeywordStatement(field, OperatorIn, func(value string) any {
		array := strings.Split(value, ",")
		for _, vt := range vts {
			if vt != nil {
				v := vt(array)
				if len(v) > 0 {
					array = v
					break
				}
			}
		}

		if len(array) == 0 {
			return nil
		}

		return array
	})
}

func KeywordIDIn(field string, vts ...ValueTransformer[[]ID, []ID]) SearchHandler {
	return KeywordStatement(field, OperatorIn, func(value string) any {
		ids := IDsFromCommaSeparatedString(value)
		for _, vt := range vts {
			if vt != nil {
				v := vt(ids)
				if len(v) > 0 {
					ids = v
					break
				}
			}
		}

		if len(ids) == 0 {
			return nil
		}

		return ids
	})
}

func KeywordLike(field string, vts ...StringValueTransformer) SearchHandler {
	return KeywordStatement(field, OperatorLike, func(value string) any {
		for _, vt := range vts {
			if vt != nil {
				v := vt(value)
				if v != "" {
					value = v
					break
				}
			}
		}

		if value == "" {
			return nil
		}

		return fmt.Sprintf("%%%s%%", value)
	})
}

func KeywordEqual(field string, vts ...StringValueTransformer) SearchHandler {
	return KeywordStatement(
		field,
		OperatorEqual,
		CastSlice(vts, func(t StringValueTransformer) AnyValueTransformer {
			return func(value string) any {
				return t(value)
			}
		})...,
	)
}

func SortBy(field string) SearchHandler {
	return func(db *gorm.DB, values []string, _ *gin.Context) (*gorm.DB, error) {
		if value, ok := PickFirstValuableString(values); ok {
			sort := "ASC"
			if strings.TrimSpace(strings.ToLower(value)) == "desc" {
				sort = "DESC"
			}
			db = db.Order(fmt.Sprintf("`%s` %s", field, sort))
		}
		return db, nil
	}
}

func NumericValidate(value string) any {
	_, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil
	}
	return value
}

func OverflowedArrayTrimmer[T any](array []T, max int) []T {
	if len(array) > max {
		return array[:max]
	}
	return array
}

func OverflowedArrayTrimmerFilter[T any](max int) func([]T) []T {
	return func(value []T) []T {
		return OverflowedArrayTrimmer(value, max)
	}
}

func GetSearchValuesFromContext(context *gin.Context) (url.Values, error) {
	var searchValues = make(url.Values)

	if context.Request.Method != http.MethodGet && context.Request.ContentLength != 0 {
		var bodyPayload map[string]string
		err := context.ShouldBindJSON(&bodyPayload) // This will panic if ContentLength is zero
		if err != nil {
			return nil, err
		}

		for key, value := range bodyPayload {
			searchValues[key] = []string{value}
		}
	}

	query := context.Request.URL.Query()
	for key, value := range query {
		// body value has higher priority
		if _, ok := searchValues[key]; ok {
			continue
		}

		// the later value has higher priority
		// for example: the last value will override previous values
		slices.Reverse(value)
		searchValues[key] = value
	}

	return searchValues, nil
}
