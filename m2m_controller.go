package gocrud

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/allape/gogger"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SetupM2MControllerOptions[T any] struct {
	// OnRecordCheck
	// use context to abort process
	OnRecordCheck func(record *T, db *gorm.DB, context *gin.Context)

	// OnDelete
	// use this to replace the original delete operation
	OnDelete func(db *gorm.DB, context *gin.Context)

	ExtraSearchHandlers SearchHandlers
}

// SetupM2MController
// M2MHelper: Many to Many, Models to Models
func SetupM2MController[T any](
	group *gin.RouterGroup, db *gorm.DB, logger *gogger.Logger,
	objectFieldName1, objectFieldName2 string,
	options *SetupM2MControllerOptions[T],
) error {
	if objectFieldName1 == "" || objectFieldName2 == "" {
		return fmt.Errorf("field1 and field2 cannot be empty")
	}

	if options == nil {
		options = &SetupM2MControllerOptions[T]{}
	}
	if options.OnRecordCheck == nil {
		options.OnRecordCheck = func(record *T, db *gorm.DB, context *gin.Context) {}
	}

	m2mHelper, err := NewM2MHelper[T](db, logger, objectFieldName1, objectFieldName2)
	if err != nil {
		return err
	}

	searchHandlers := MergeSearchHandlers(m2mHelper.SearchHandlers, options.ExtraSearchHandlers)

	var getAllHandler gin.HandlerFunc = func(context *gin.Context) {
		var err error

		repo := db.Model(new(T))

		repo, err = HandleSearch(context, repo, searchHandlers)
		if err != nil {
			MakeErrorResponse(context, RestCoder.BadRequest(), "[error] failed to handle search")
			return
		}

		m2mHelper.CheckIfInSearchHandled(context)
		if context.IsAborted() {
			return
		}

		var list []T
		if err := repo.Find(&list).Error; err != nil {
			logger.Error().Printf("failed to get list: %v", err)
			MakeErrorResponse(context, RestCoder.InternalServerError(), "[error] failed to get list")
			return
		}

		MakeOkayDataResponse(context, list)
	}

	group.GET("/all", getAllHandler)
	group.POST("/all", getAllHandler)

	// batch save
	group.PUT("/save", func(context *gin.Context) {
		var records []T
		if err := context.ShouldBindJSON(&records); err != nil {
			MakeErrorResponse(context, RestCoder.BadRequest(), "[error] failed to parse body")
			return
		}

		for index, record := range records {
			id1, id2, err := m2mHelper.ExtractIDsFromRecord(&record)
			if err != nil {
				MakeErrorResponse(context, RestCoder.BadRequest(), "[error] failed to extract ids")
				return
			}
			if id1 == 0 {
				MakeErrorResponse(context, RestCoder.BadRequest(), fmt.Sprintf("%s can not be 0 at %d", m2mHelper.JSONFieldName1, index))
				return
			}
			if id2 == 0 {
				MakeErrorResponse(context, RestCoder.BadRequest(), fmt.Sprintf("%s can not be 0 at %d", m2mHelper.JSONFieldName2, index))
				return
			}

			options.OnRecordCheck(&records[index], db, context)
			if context.IsAborted() {
				return
			}
		}

		res := db.Save(&records)

		if err := res.Error; err != nil {
			logger.Error().Printf("failed to save record: %v", err)
			return
		}

		MakeOkayDataResponse(context, res.RowsAffected)
	})

	group.POST("/save/:deleteByField/:deleteById", func(context *gin.Context) {
		deleteByField := strings.TrimSpace(context.Param("deleteByField"))
		if deleteByField != m2mHelper.JSONFieldName1 && deleteByField != m2mHelper.JSONFieldName2 {
			MakeErrorResponse(context, RestCoder.BadRequest(), "field for delete is invalid")
			return
		}

		deleteById, err := ParseIDParam(context, "deleteById")
		if err != nil {
			MakeErrorResponse(context, RestCoder.BadRequest(), "id for delete is invalid")
			return
		} else if deleteById == 0 {
			MakeErrorResponse(context, RestCoder.BadRequest(), "id for delete can not be 0")
			return
		}

		var records []T
		if err := context.ShouldBindJSON(&records); err != nil {
			MakeErrorResponse(context, RestCoder.BadRequest(), "invalid request body")
			return
		}

		var objectPrimaryFieldName string
		var dbFieldName string

		switch deleteByField {
		case m2mHelper.JSONFieldName1:
			objectPrimaryFieldName = objectFieldName1
			dbFieldName = m2mHelper.DatabaseFieldName1
		case m2mHelper.JSONFieldName2:
			objectPrimaryFieldName = objectFieldName2
			dbFieldName = m2mHelper.DatabaseFieldName2
		}

		for i, record := range records {
			reflected := reflect.ValueOf(record)
			idField := reflected.FieldByName(objectPrimaryFieldName)
			id := idField.Uint()
			if id != uint64(deleteById) {
				MakeErrorResponse(context, RestCoder.BadRequest(), fmt.Sprintf("id of record at %d is invalid, expect %d, but got %d", i, deleteById, id))
				return
			}

			options.OnRecordCheck(&records[i], db, context)
			if context.IsAborted() {
				return
			}
		}

		count := int64(0)

		err = db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Delete(new(T), fmt.Sprintf("`%s` = ?", dbFieldName), deleteById).Error; err != nil {
				return err
			}

			if len(records) == 0 {
				return nil
			}

			res := tx.Save(records)
			if res.Error != nil {
				return res.Error
			}

			count = res.RowsAffected

			return nil
		})
		if err != nil {
			logger.Error().Printf("failed to save %v for %s of %d: %v", records, deleteByField, deleteById, err)
			MakeErrorResponse(context, RestCoder.InternalServerError(), "[error] failed to save")
			return
		}

		MakeOkayDataResponse(context, count)
	})

	// ?[jsonFieldName1]=id1&[jsonFieldName2]=id2
	group.DELETE("", func(context *gin.Context) {
		if options.OnDelete != nil {
			options.OnDelete(db, context)
			return
		}
		m2mHelper.OnDelete(context)
	})

	return nil
}
