package gocrud

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/url"
	"time"
)

type ID uint64

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

func HandleSoftDeleteSearch(db *gorm.DB, values []string, _ url.Values) *gorm.DB {
	if ok, deleted := ValuableArray(values); ok {
		if deleted == "false" {
			db = db.Where("deleted_at IS NULL")
		} else {
			db = db.Where("deleted_at IS NOT NULL")
		}
	}
	return db
}
