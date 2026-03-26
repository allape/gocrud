package gocrud

import (
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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
		if deleted, ok := PickFirstValuableString(values); ok {
			if deleted == "false" {
				db = db.Where(fmt.Sprintf("%s IS NULL", fieldName))
			} else {
				db = db.Where(fmt.Sprintf("%s IS NOT NULL", fieldName))
			}
		}
		return db
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

type DuplicateFieldCheckResult bool

const (
	Okay    DuplicateFieldCheckResult = true
	NotOkay DuplicateFieldCheckResult = false
)

// DuplicateFieldCheck
// T must extend from gocrud.Base which must contain id field
func DuplicateFieldCheck[T any](db *gorm.DB, context *gin.Context, objectForCheck *T, objectFieldName, dbFieldName string) (DuplicateFieldCheckResult, error) {
	record := reflect.ValueOf(objectForCheck).Elem()

	valueField := record.FieldByName(objectFieldName)
	idField := record.FieldByName("ID")

	valueForCheck := record.FieldByName(objectFieldName).String()

	if !valueField.IsValid() || valueForCheck == "" {
		MakeErrorResponse(context, RestCoder.InternalServerError(), "record is invalid")
		return NotOkay, fmt.Errorf("there is no valid value in field %s", objectFieldName)
	}

	id := uint64(0)
	if idField.CanUint() {
		id = idField.Uint()
	}

	if id > 0 {
		var old T
		if err := db.Model(&old).Where("id = ?", id).First(&old).Error; err != nil {
			MakeErrorResponse(context, RestCoder.NotFound(), "record not found")
			return NotOkay, fmt.Errorf("unable to find old record for id [%d]", id)
		}

		oldValue := reflect.ValueOf(old).FieldByName(objectFieldName).String()

		if oldValue == valueForCheck {
			valueForCheck = ""
		}
	}

	if valueForCheck != "" {
		var m T
		var count int64
		if err := db.Model(&m).Where(fmt.Sprintf("`%s` = ?", dbFieldName), valueForCheck).Count(&count).Error; err != nil {
			MakeErrorResponse(context, RestCoder.InternalServerError(), fmt.Sprintf("%s is invalid", objectFieldName))
			return NotOkay, fmt.Errorf("%s [%s] duplication check failed: [%v]", objectFieldName, valueForCheck, err)
		} else if count > 0 {
			msg := fmt.Sprintf("%s [%s] has been taken", objectFieldName, valueForCheck)
			MakeErrorResponse(context, RestCoder.BadRequest(), msg)
			return NotOkay, errors.New(msg)
		}
	}

	return Okay, nil
}
