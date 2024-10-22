package gocrud

import (
	"fmt"
	"gorm.io/gorm"
	"net/url"
	"strings"
)

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
		if len(values) == 0 {
			return
		}

		if filterFunc != nil {
			values = filterFunc(values)
		}

		if len(values) == 0 {
			return
		}

		db.Where(
			fmt.Sprintf("`%s` in ?", name),
			values,
		)
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
