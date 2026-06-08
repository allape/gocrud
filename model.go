package gocrud

import (
	"errors"
	"fmt"
	"maps"
	"reflect"
	"strconv"
	"time"

	"github.com/allape/gogger"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ID uint64

var IDKind = reflect.Uint64

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

// DuplicateFieldCheck
// T must extend from Base which must contain id field
func DuplicateFieldCheck[T any](
	db *gorm.DB, context *gin.Context, logger *gogger.Logger,
	objectForCheck *T, objectFieldName, dbFieldName string,
) error {
	record := reflect.ValueOf(objectForCheck).Elem()

	valueField := record.FieldByName(objectFieldName)
	idField := record.FieldByName("ID")

	valueForCheck := record.FieldByName(objectFieldName).String()

	if !valueField.IsValid() || valueForCheck == "" {
		MakeErrorResponse(context, RestCoder.InternalServerError(), "[error] record is invalid")
		err := fmt.Errorf("there is no valid value in field %s", objectFieldName)
		logger.Error().Print(err.Error())
		return err
	}

	id := uint64(0)
	if idField.CanUint() {
		id = idField.Uint()
	}

	if id > 0 {
		var old T
		if err := db.Model(&old).Where("id = ?", id).First(&old).Error; err != nil {
			MakeErrorResponse(context, RestCoder.NotFound(), "record not found")
			return fmt.Errorf("unable to find old record for id [%d]", id)
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
			MakeErrorResponse(context, RestCoder.InternalServerError(), fmt.Sprintf("[error] %s is invalid", objectFieldName))
			logger.Error().Printf("%s [%s] duplication check failed: [%v]", objectFieldName, valueForCheck, err)
			return err
		} else if count > 0 {
			msg := fmt.Sprintf("%s [%s] has been taken", objectFieldName, valueForCheck)
			MakeErrorResponse(context, RestCoder.BadRequest(), msg)
			return errors.New(msg)
		}
	}

	return nil
}
