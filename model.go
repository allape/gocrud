package gocrud

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type ID uint64

type IBase interface {
	GetID() ID
}

type Base struct {
	ID        ID         `json:"id"        gorm:"primaryKey"`
	CreatedAt time.Time  `json:"createdAt" gorm:"autoCreateTime;<-:create"`
	UpdatedAt time.Time  `json:"updatedAt" gorm:"autoUpdateTime"`
	DeletedAt *time.Time `json:"deletedAt"`
}

func NewHardDeleteHandler[T any](coder Coder) func(context *gin.Context, db *gorm.DB) bool {
	var record T
	return func(context *gin.Context, db *gorm.DB) bool {
		id := context.Param("id")
		if id == "" {
			MakeErrorResponse(context, coder.BadRequest(), "invalid ID")
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
			MakeErrorResponse(context, coder.BadRequest(), "invalid ID")
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

	return func(db *gorm.DB, values []string, _ url.Values) *gorm.DB {
		if ok, deleted := ValuableArray(values); ok {
			if deleted == "false" {
				db = db.Where(fmt.Sprintf("%s IS NULL", fieldName))
			} else {
				db = db.Where(fmt.Sprintf("%s IS NOT NULL", fieldName))
			}
		}
		return db
	}
}

func IDsFromCommaSplitString(css string) []ID {
	var ids []ID
	for _, s := range strings.Split(css, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}

		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			continue
		}

		ids = append(ids, ID(id))
	}
	return ids
}

func ContainsByBase[T IBase](array []T, one T) bool {
	for _, item := range array {
		if item.GetID() == one.GetID() {
			return true
		}
	}
	return false
}

func ContainsByID[T IBase](array []T, id ID) bool {
	for _, item := range array {
		if item.GetID() == id {
			return true
		}
	}
	return false
}
