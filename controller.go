package gocrud

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/allape/gogger"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func SetupDualPrimaryKeyModelController[T any](
	group *gin.RouterGroup, db *gorm.DB, logger *gogger.Logger,
	objectFieldName1, objectFieldName2 string,
	databaseFieldName1, databaseFieldName2 string,
) error {
	if objectFieldName1 == "" || objectFieldName2 == "" {
		return fmt.Errorf("field1 and field2 cannot be empty")
	}

	var jsonFieldName1, jsonFieldName2 string

	// setup check
	// use a block to drop them after check is done
	{
		record := new(T)
		reflected := reflect.TypeOf(record).Elem()

		ofn1, ok := reflected.FieldByName(objectFieldName1)
		if !ok {
			return fmt.Errorf("field %s is invalid", objectFieldName1)
		} else if ofn1.Type.Kind() != IDKind {
			return fmt.Errorf("type of field %s is invalid, should be %d", objectFieldName1, IDKind)
		}

		ofn2, ok := reflected.FieldByName(objectFieldName2)
		if !ok {
			return fmt.Errorf("field %s is invalid", objectFieldName2)
		} else if ofn2.Type.Kind() != IDKind {
			return fmt.Errorf("type of field %s is invalid, should be %d", objectFieldName2, IDKind)
		}

		jsonFieldName1 = GetJSONFieldName(record, objectFieldName1)
		if jsonFieldName1 == "" {
			return fmt.Errorf("failed to get the json field name of %s", objectFieldName1)
		}

		jsonFieldName2 = GetJSONFieldName(record, objectFieldName2)
		if jsonFieldName2 == "" {
			return fmt.Errorf("failed to get the json field name of %s", objectFieldName2)
		}
	}

	inFieldName1 := "in_" + jsonFieldName1
	inFieldName2 := "in_" + jsonFieldName2

	inField1WhereStatement := fmt.Sprintf("`%s` IN ?", databaseFieldName1)
	inField2WhereStatement := fmt.Sprintf("`%s` IN ?", databaseFieldName2)

	group.GET("/all", func(context *gin.Context) {
		f1 := IDsFromCommaSeparatedString(context.Query(inFieldName1))
		f2 := IDsFromCommaSeparatedString(context.Query(inFieldName2))
		if len(f1) == 0 && len(f2) == 0 {
			MakeErrorResponse(context, RestCoder.BadRequest(), "ids cannot be empty")
			return
		}

		repo := db.Model(new(T))

		if len(f1) > 0 {
			repo = repo.Where(inField1WhereStatement, f1)
		}

		if len(f2) > 0 {
			repo = repo.Where(inField2WhereStatement, f2)
		}

		var list []T
		if err := repo.Find(&list).Error; err != nil {
			logger.Error().Printf("failed to get list: %v", err)
			MakeErrorResponse(context, RestCoder.InternalServerError(), "[error] failed to get list")
			return
		}

		MakeOkayDataResponse(context, list)
	})

	group.PUT("/save", func(context *gin.Context) {
		var record T
		if err := context.ShouldBind(&record); err != nil {
			MakeErrorResponse(context, RestCoder.BadRequest(), "[error] failed to parse body")
			return
		}

		reflected := reflect.ValueOf(record)

		id1 := reflected.FieldByName(objectFieldName1).Uint()
		if id1 == 0 {
			MakeErrorResponse(context, RestCoder.BadRequest(), fmt.Sprintf("%s can not be 0", jsonFieldName1))
			return
		}

		id2 := reflected.FieldByName(objectFieldName2).Uint()
		if id2 == 0 {
			MakeErrorResponse(context, RestCoder.BadRequest(), fmt.Sprintf("%s can not be 0", jsonFieldName2))
			return
		}

		if err := db.Save(&record).Error; err != nil {
			logger.Error().Printf("failed to save record: %v", err)
			return
		}

		MakeOkayDataResponse(context, record)
	})

	group.POST("/save/:deletedBy/:deletedId", func(context *gin.Context) {
		deletedBy := strings.TrimSpace(context.Param("deletedBy"))
		if deletedBy != jsonFieldName1 && deletedBy != jsonFieldName2 {
			MakeErrorResponse(context, RestCoder.BadRequest(), "field for delete is invalid")
			return
		}

		deletedId, err := strconv.ParseUint(context.Param("deletedId"), 10, 64)
		if err != nil {
			MakeErrorResponse(context, RestCoder.BadRequest(), "id for delete is invalid")
			return
		} else if deletedId == 0 {
			MakeErrorResponse(context, RestCoder.BadRequest(), "id for delete can not be 0")
			return
		}

		var records []T
		if err := context.ShouldBind(&records); err != nil {
			MakeErrorResponse(context, RestCoder.BadRequest(), "invalid request body")
			return
		}

		var objectPrimaryFieldName string
		var dbFieldName string

		switch deletedBy {
		case jsonFieldName1:
			objectPrimaryFieldName = objectFieldName1
			dbFieldName = databaseFieldName1
		case jsonFieldName2:
			objectPrimaryFieldName = objectFieldName2
			dbFieldName = databaseFieldName2
		}

		for i, record := range records {
			reflected := reflect.ValueOf(record)
			idField := reflected.FieldByName(objectPrimaryFieldName)
			id := idField.Uint()
			if id != deletedId {
				MakeErrorResponse(context, RestCoder.BadRequest(), fmt.Sprintf("id of record at %d is invalid, expect %d, but got %d", i, deletedId, id))
				return
			}
		}

		count := int64(0)

		err = db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Delete(new(T), fmt.Sprintf("`%s` = ?", dbFieldName), deletedId).Error; err != nil {
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
			logger.Error().Printf("failed to save %v for %s of %d: %v", records, deletedBy, deletedId, err)
			MakeErrorResponse(context, RestCoder.InternalServerError(), "[error] failed to save")
			return
		}

		MakeOkayDataResponse(context, count)
	})

	// ?[jsonFieldName1]=id1&[jsonFieldName2]=id2
	group.DELETE("", func(context *gin.Context) {
		id1, err := strconv.ParseUint(context.Query(jsonFieldName1), 10, 64)
		if err != nil {
			MakeErrorResponse(context, RestCoder.BadRequest(), fmt.Sprintf("value of %s is invalid", jsonFieldName1))
			return
		}
		id2, err := strconv.ParseUint(context.Query(jsonFieldName2), 10, 64)
		if err != nil {
			MakeErrorResponse(context, RestCoder.BadRequest(), fmt.Sprintf("value of %s is invalid", jsonFieldName2))
			return
		}

		res := db.Delete(new(T), fmt.Sprintf("`%s` = ? AND `%s` = ?", databaseFieldName1, databaseFieldName2), id1, id2)
		if res.Error != nil {
			logger.Error().Printf("failed to delete at %d,%d: %v", id1, id2, res.Error)
			MakeErrorResponse(context, RestCoder.InternalServerError(), "[error] failed to delete")
			return
		}

		MakeOkayDataResponse(context, res.RowsAffected)
	})

	return nil
}
