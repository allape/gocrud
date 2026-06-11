package gocrud

import (
	"fmt"
	"maps"
	"net/http"
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
	extraSearchHandlers ...SearchHandlers,
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

		jsonFields, err := GetJSONFieldNameOf[T](objectFieldName1, objectFieldName2)
		if err != nil || len(jsonFields) != 2 {
			return fmt.Errorf("failed to get JSON fields: %v", err)
		}

		jsonFieldName1 = jsonFields[0]
		jsonFieldName2 = jsonFields[1]
	}

	inFieldName1 := "in_" + jsonFieldName1
	inFieldName2 := "in_" + jsonFieldName2

	// used to mark if inFieldName1 or inFieldName2 has been triggered
	const ContextKeyForHandledKeywordIn = "gocrud:dpkm:didkeywordin"

	var handleKeywordIdIn = func(databaseFieldName string) SearchHandler {
		return func(db *gorm.DB, values []string, context *gin.Context) (*gorm.DB, error) {
			return KeywordIDIn(databaseFieldName, func(value []ID) []ID {
				if len(value) > 0 {
					context.Set(ContextKeyForHandledKeywordIn, true)
				}
				return value
			})(db, values, context)
		}
	}

	searchHandlers := SearchHandlers{
		inFieldName1: handleKeywordIdIn(databaseFieldName1),
		inFieldName2: handleKeywordIdIn(databaseFieldName2),
	}
	for _, handlers := range extraSearchHandlers {
		maps.Insert(searchHandlers, maps.All(handlers))
	}

	var getAllHandler gin.HandlerFunc = func(context *gin.Context) {
		var err error

		repo := db.Model(new(T))

		repo, err = HandleSearch(context, repo, searchHandlers)
		if err != nil {
			MakeErrorResponse(context, RestCoder.BadRequest(), "[error] failed to handle search")
			return
		}

		_, ok := context.Get(ContextKeyForHandledKeywordIn)
		if !ok {
			MakeErrorResponse(context, RestCoder.BadRequest(), fmt.Sprintf("at least one of %s or %s should not be empty", inFieldName1, inFieldName2))
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

// DualPrimaryKeyModelHandler
// T1: model 1, should have ID field, connected by objectFieldName1 of DPKM
// T2: model 2, should have ID field, connected by objectFieldName2 of DPKM
// DPKM: dual primary key model
type DualPrimaryKeyModelHandler[T1 any, T2 any, DPKM any] struct {
	httpClient          *http.Client
	okayHttpStatusRange *HttpStatusRange

	objectFieldName1   string
	objectFieldName2   string
	databaseFieldName1 string
	databaseFieldName2 string
	jsonFieldName1     string
	jsonFieldName2     string
}

func (d *DualPrimaryKeyModelHandler[T1, T2, DPKM]) GetAll() {
	// TODO
}

func NewDualPrimaryKeyModelHandler[T1 any, T2 any, DPKM any](
	httpClient *http.Client,
	okayHttpStatusRange *HttpStatusRange,
	objectFieldName1, objectFieldName2 string,
	databaseFieldName1, databaseFieldName2 string,
) (*DualPrimaryKeyModelHandler[T1, T2, DPKM], error) {
	if objectFieldName1 == "" {
		return nil, fmt.Errorf("objectFieldName1 is empty")
	}
	if objectFieldName2 == "" {
		return nil, fmt.Errorf("objectFieldName2 is empty")
	}
	if databaseFieldName1 == "" {
		return nil, fmt.Errorf("databaseFieldName1 is empty")
	}
	if databaseFieldName2 == "" {
		return nil, fmt.Errorf("databaseFieldName2 is empty")
	}

	jsonFields, err := GetJSONFieldNameOf[DPKM](objectFieldName1, objectFieldName2)
	if err != nil {
		return nil, err
	} else if len(jsonFields) != 2 {
		return nil, fmt.Errorf("expect 2 json fields, got %d", len(jsonFields))
	}

	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if okayHttpStatusRange == nil {
		okayHttpStatusRange = &DefaultOkayHttpStatusRange
	}

	handler := &DualPrimaryKeyModelHandler[T1, T2, DPKM]{
		httpClient:          httpClient,
		okayHttpStatusRange: okayHttpStatusRange,
		objectFieldName1:    objectFieldName1,
		objectFieldName2:    objectFieldName2,
		databaseFieldName1:  databaseFieldName1,
		databaseFieldName2:  databaseFieldName2,

		jsonFieldName1: jsonFields[0],
		jsonFieldName2: jsonFields[1],
	}
	return handler, nil
}
