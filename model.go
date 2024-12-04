package gocrud

import (
	"errors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/url"
	"time"
)

type Base struct {
	ID        uint64     `json:"id"        gorm:"primaryKey"`
	CreatedAt time.Time  `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt time.Time  `json:"updatedAt" gorm:"autoUpdateTime"`
	DeletedAt *time.Time `json:"deletedAt"`
}

func NewSoftDeleteHandler[T any]() func(ctx *gin.Context, repo *gorm.DB) (bool, error) {
	var record T
	return func(context *gin.Context, repo *gorm.DB) (bool, error) {
		id := context.Query("id")

		if id == "" {
			return false, errors.New("invalid ID")
		}

		res := repo.Model(record).Where("id = ?", id).UpdateColumn("deleted_at", time.Now())

		return res.RowsAffected > 0, res.Error
	}
}

func HandleSoftDeleteSearch(db *gorm.DB, values []string, _ url.Values) {
	if ok, deleted := ValuableStringFromArray(values); ok {
		if deleted == "false" {
			db = db.Where("deleted_at is null")
		} else {
			db = db.Where("deleted_at is not null")
		}
	}
}
