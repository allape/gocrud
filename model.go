package gocrud

import (
	"errors"
	"fmt"
	"reflect"
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

func NewPrioritySearchHandlers() SearchHandlers {
	return SearchHandlers{
		"orderBy_priority": SortBy("priority"),
		// sortByPriorityThenUpdatedAt: true|false
		"sortByPriorityThenUpdatedAt": func(db *gorm.DB, values []string, _ *gin.Context) (*gorm.DB, error) {
			if doSort, ok := PickFirstValuableString(values); ok {
				if doSort != "false" {
					return db.Order("`priority` DESC, `updated_at` DESC"), nil
				}
			}
			return db, nil
		},
	}
}

func NewBaseSearchHandlers(extra ...SearchHandlers) SearchHandlers {
	base := MergeSearchHandlers(
		SearchHandlers{
			"in_id":   KeywordIDIn("id", nil),
			"deleted": NewSoftDeleteSearchHandler(""),

			"orderBy_createdAt": SortBy("created_at"),
			"orderBy_updatedAt": SortBy("updated_at"),
			"orderBy_deletedAt": SortBy("deleted_at"),
		},
		NewPrioritySearchHandlers(),
	)
	return MergeSearchHandlers(base, extra...)
}

func NewHardDeleteHandler[T any](coder Coder) func(context *gin.Context, db *gorm.DB) bool {
	return func(context *gin.Context, db *gorm.DB) bool {
		id, err := ParseIDParam(context, "id")
		if err != nil {
			MakeErrorResponse(context, coder.BadRequest(), "[error] invalid id")
			return false
		} else if id == 0 {
			MakeErrorResponse(context, coder.BadRequest(), "invalid id")
			return false
		}

		res := db.Delete(new(T), "id = ?", id)
		if res.Error != nil {
			MakeErrorResponse(context, coder.InternalServerError(), res)
			return false
		}

		return res.RowsAffected > 0
	}
}

func NewSoftDeleteHandler[T any](coder Coder) func(context *gin.Context, db *gorm.DB) bool {
	return func(context *gin.Context, db *gorm.DB) bool {
		id, err := ParseIDParam(context, "id")
		if err != nil {
			MakeErrorResponse(context, coder.BadRequest(), "[error] invalid id")
			return false
		} else if id == 0 {
			MakeErrorResponse(context, coder.BadRequest(), "invalid id")
			return false
		}

		res := db.Model(new(T)).Where("id = ?", id).UpdateColumn("deleted_at", time.Now())
		if res.Error != nil {
			MakeErrorResponse(context, coder.InternalServerError(), res)
			return false
		}

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
		id, err := ParseID(s)
		if err != nil {
			return
		}
		ids = append(ids, id)
	}, css)
	return ids
}

func ParseID(idStr string) (ID, error) {
	id, err := ParseUint64(idStr)
	if err != nil {
		return 0, err
	}
	return ID(id), nil
}

func ParseIDParam(context *gin.Context, name string) (ID, error) {
	idStr := context.Param(name)
	if idStr == "" {
		return 0, errors.New("empty id")
	}
	return ParseID(idStr)
}

// NewDuplicateFieldCheckFunc
// T must extend from Base which must contain id field
func NewDuplicateFieldCheckFunc[T any](
	db *gorm.DB, logger *gogger.Logger,
	objectFieldName string,
) (func(context *gin.Context, objectForCheck *T) error, error) {
	var dbFieldName string

	// runtime check
	{
		dbFieldNames, err := GetDatabaseFieldNameOf[T](db, objectFieldName)
		if err != nil {
			return nil, err
		} else if len(dbFieldNames) != 1 {
			return nil, fmt.Errorf("database field for object field %s not found", objectFieldName)
		}

		dbFieldName = dbFieldNames[0]
	}

	return func(context *gin.Context, objectForCheck *T) error {
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
			var oldOnes []T
			if err := db.Model(new(T)).Where("id = ?", id).Limit(1).Find(&oldOnes).Error; err != nil {
				MakeErrorResponse(context, RestCoder.InternalServerError(), "failed to find old records")
				return fmt.Errorf("unable to find old records for id %d", id)
			}

			if len(oldOnes) > 0 {
				old := oldOnes[0]
				oldValue := reflect.ValueOf(old).FieldByName(objectFieldName).String()
				if oldValue == valueForCheck {
					valueForCheck = ""
				}
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
	}, nil
}
