package gocrud

import (
	"fmt"
	"maps"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ID uint64

type Base struct {
	ID        ID         `json:"id"        gorm:"primaryKey"`
	Priority  int64      `json:"priority"`
	CreatedAt time.Time  `json:"createdAt" gorm:"autoCreateTime;<-:create"`
	UpdatedAt time.Time  `json:"updatedAt" gorm:"autoUpdateTime"`
	DeletedAt *time.Time `json:"deletedAt"`
}

func BaseSearchHandlers(overrideSearchHandlers ...SearchHandlers) SearchHandlers {
	base := map[string]SearchHandler{
		"in_id":             KeywordIDIn("id", nil),
		"orderBy_createdAt": SortBy("created_at"),
		"orderBy_updatedAt": SortBy("updated_at"),
		"orderBy_deletedAt": SortBy("deleted_at"),
		"deleted":           NewSoftDeleteSearchHandler(""),

		"orderBy_priority": SortBy("priority"),
		// `order by` must be followed by `ASC` or `DESC`
		// `sort by` has defined the order
		"sortByPriorityThenUpdatedAt": func(db *gorm.DB, values []string, _ *gin.Context) (*gorm.DB, error) {
			return db.Order("`priority` DESC, `updated_at` DESC"), nil
		},
	}
	for _, handlers := range overrideSearchHandlers {
		maps.Insert(base, maps.All(handlers))
	}
	return base
}

func NewHardDeleteHandler[T any](coder Coder) func(context *gin.Context, db *gorm.DB) bool {
	var record T
	return func(context *gin.Context, db *gorm.DB) bool {
		id := context.Param("id")
		if id == "" {
			MakeErrorResponse(context, coder.BadRequest(), "invalid id")
			return false
		}

		res := db.Delete(&record, id)

		return res.RowsAffected > 0
	}
}

func NewSoftDeleteHandler[T any](coder Coder) func(context *gin.Context, db *gorm.DB) bool {
	var record T
	return func(context *gin.Context, db *gorm.DB) bool {
		id := context.Param("id")

		if id == "" {
			MakeErrorResponse(context, coder.BadRequest(), "invalid id")
			return false
		}

		res := db.Model(&record).Where("id = ?", id).UpdateColumn("deleted_at", time.Now())

		return res.RowsAffected > 0
	}
}

func NewSoftDeleteSearchHandler(tableName string) SearchHandler {
	fieldName := "`deleted_at`"
	if tableName != "" {
		fieldName = fmt.Sprintf("`%s`.%s", tableName, fieldName)
	}

	return func(db *gorm.DB, values []string, _ *gin.Context) (*gorm.DB, error) {
		if deleted, ok := PickFirstValuableString(values); ok {
			if deleted == "false" {
				db = db.Where(fmt.Sprintf("%s IS NULL", fieldName))
			} else {
				db = db.Where(fmt.Sprintf("%s IS NOT NULL", fieldName))
			}
		}
		return db, nil
	}
}

func IDsFromCommaSeparatedString(css string) []ID {
	var ids []ID
	MapFuncOverCommaSeparatedString(func(s string) {
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return
		}
		ids = append(ids, ID(id))
	}, css)
	return ids
}
