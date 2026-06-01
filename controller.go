package gocrud

import (
	"errors"
	"fmt"
	"reflect"
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

	// startup check
	// use a block to drop them after check is done
	{
		record := new(T)
		reflected := reflect.TypeOf(record).Elem()

		if _, ok := reflected.FieldByName(objectFieldName1); !ok {
			return fmt.Errorf("field %s is invalid", objectFieldName1)
		}

		if _, ok := reflected.FieldByName(objectFieldName2); !ok {
			return fmt.Errorf("field %s is invalid", objectFieldName2)
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

	// /save/[jsonField1 || jsonField2]?[jsonField1]=1,2,3...&[jsonField2]=1,2,3...
	// delete by primaryFieldName before save
	group.POST("/save/:primaryFieldName", func(c *gin.Context) {
		primaryFieldName := strings.TrimSpace(c.Param("primaryFieldName"))
		if primaryFieldName != jsonFieldName1 && primaryFieldName != jsonFieldName2 {
			MakeErrorResponse(c, RestCoder.BadRequest(), "field name invalid")
			return
		}

		field1Ids := IDsFromCommaSeparatedString(c.Query(jsonFieldName1))
		field2Ids := IDsFromCommaSeparatedString(c.Query(jsonFieldName2))

		var primaryId ID
		var secondaryIds []ID

		var objectPrimaryFieldName string
		var objectSecondaryFieldName string
		var dbFieldName string

		switch primaryFieldName {
		case jsonFieldName1:
			primaryId = Pick(field1Ids, 0, 0)
			secondaryIds = field2Ids
			objectPrimaryFieldName = objectFieldName1
			objectSecondaryFieldName = objectFieldName2
			dbFieldName = databaseFieldName1
		case jsonFieldName2:
			primaryId = Pick(field2Ids, 0, 0)
			secondaryIds = field1Ids
			objectPrimaryFieldName = objectFieldName2
			objectSecondaryFieldName = objectFieldName1
			dbFieldName = databaseFieldName2
		}

		if primaryId == 0 {
			MakeErrorResponse(c, RestCoder.BadRequest(), "pid cannot be empty")
			return
		}

		secondaryIds = RemoveDuplication(secondaryIds)

		count := int64(0)

		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Delete(new(T), fmt.Sprintf("`%s` = ?", dbFieldName), primaryId).Error; err != nil {
				return err
			}

			if len(secondaryIds) == 0 {
				return nil
			}

			items := make([]*T, len(secondaryIds))
			for i, sid := range secondaryIds {
				record := new(T)

				reflected := reflect.ValueOf(record).Elem()

				primaryField := reflected.FieldByName(objectPrimaryFieldName)
				primaryField.SetUint(uint64(primaryId))

				secondaryField := reflected.FieldByName(objectSecondaryFieldName)
				secondaryField.SetUint(uint64(sid))

				items[i] = record
			}

			res := tx.Save(items)
			if res.Error != nil {
				return res.Error
			}

			count = res.RowsAffected

			return nil
		})
		if err != nil {
			logger.Error().Printf("failed to save %v for %d: %v", secondaryIds, primaryId, err)
			MakeErrorResponse(c, RestCoder.InternalServerError(), "[error] failed to save")
			return
		}

		MakeOkayDataResponse(c, count)
	})

	return nil
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
