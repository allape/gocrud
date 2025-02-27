package gocrud

import (
	"fmt"
	"gorm.io/gorm"
	"net/url"
	"strconv"
	"strings"
)

type (
	SearchHandler  = func(db *gorm.DB, values []string, with url.Values) *gorm.DB
	SearchHandlers = map[string]SearchHandler
)

type (
	Operator                       string
	ValueTransformer[T any, R any] func(value T) R
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

func KeywordStatement(name string, operator Operator, vt ValueTransformer[string, any]) SearchHandler {
	return func(db *gorm.DB, values []string, _ url.Values) *gorm.DB {
		if ok, value := ValuableArray(values); ok {
			var anyValue any = value
			if vt != nil {
				anyValue = vt(value)
				if anyValue == nil {
					return db
				}
			}

			// cheat
			if operator == OperatorIn || operator == OperatorNotIn {
				if arr, ok := anyValue.([]any); ok && len(arr) == 0 {
					return db.Where("1 != 1")
				}
			}

			db = db.Where(
				fmt.Sprintf("`%s` %s ?", name, operator),
				anyValue,
			)
		}
		return db
	}
}

func KeywordIn(name string, vt ValueTransformer[[]string, []string]) SearchHandler {
	return KeywordStatement(name, OperatorIn, func(value string) any {
		array := strings.Split(value, ",")
		if vt != nil {
			array = vt(array)
			if len(array) == 0 {
				return nil
			}
		}
		return array
	})
}

func KeywordIDIn(name string, vt ValueTransformer[[]ID, []ID]) SearchHandler {
	return KeywordStatement(name, OperatorIn, func(value string) any {
		ids := IDsFromCommaSplitString(value)
		if vt != nil {
			ids = vt(ids)
			if len(ids) == 0 {
				return nil
			}
		}
		return ids
	})
}

func KeywordLike(name string, vt ValueTransformer[string, any]) SearchHandler {
	return KeywordStatement(name, OperatorLike, func(value string) any {
		var anyValue any = value
		if vt != nil {
			anyValue = vt(value)
		}
		return fmt.Sprintf("%%%s%%", anyValue)
	})
}

func KeywordEqual(name string, vt ValueTransformer[string, any]) SearchHandler {
	return KeywordStatement(name, OperatorEqual, vt)
}

func SortBy(name string) SearchHandler {
	return func(db *gorm.DB, values []string, _ url.Values) *gorm.DB {
		if ok, value := ValuableArray(values); ok {
			sort := "asc"
			if strings.ToLower(value) == "desc" {
				sort = "desc"
			}
			db = db.Order(fmt.Sprintf("`%s` %s", name, sort))
		}
		return db
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
