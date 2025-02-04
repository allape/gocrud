package gocrud

import (
	"fmt"
	"gorm.io/gorm"
	"net/url"
	"strconv"
	"strings"
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
			array = vt(strings.Split(value, ","))
			if len(array) == 0 {
				return nil
			}
		}
		return array
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
