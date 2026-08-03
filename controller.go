package gocrud

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/allape/gogger"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SetupM2MConnectorController
// M2M: Many to Many, Models to Models
func SetupM2MConnectorController[T any](
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
	const ContextKeyForHandledKeywordIn = "gocrud:m2mc:didkeywordin"

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
		var records []T
		if err := context.ShouldBind(&records); err != nil {
			MakeErrorResponse(context, RestCoder.BadRequest(), "[error] failed to parse body")
			return
		}

		for index, record := range records {
			reflected := reflect.ValueOf(record)

			id1 := reflected.FieldByName(objectFieldName1).Uint()
			if id1 == 0 {
				MakeErrorResponse(context, RestCoder.BadRequest(), fmt.Sprintf("%s can not be 0 at %d", jsonFieldName1, index))
				return
			}

			id2 := reflected.FieldByName(objectFieldName2).Uint()
			if id2 == 0 {
				MakeErrorResponse(context, RestCoder.BadRequest(), fmt.Sprintf("%s can not be 0 at %d", jsonFieldName2, index))
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

// M2MConnectorHandler
// M1: model 1, should have ID field, connected by objectFieldName1 of M2MConnector
// M2: model 2, should have ID field, connected by objectFieldName2 of M2MConnector
// M2M: Many to Many, Models to Models
type M2MConnectorHandler[M1 any, M2 any, M2MConnector any] struct {
	baseURL             string
	httpClient          *http.Client
	okayHttpStatusRange *HttpStatusRange

	ObjectFieldName1 string
	ObjectFieldName2 string

	jsonFieldName1 string
	jsonFieldName2 string
}

func (d *M2MConnectorHandler[M1, M2, M2MConnector]) GetAll(t1IDs, t2IDs []ID, params ...SearchParams) ([]M2MConnector, error) {
	if len(t1IDs) == 0 && len(t2IDs) == 0 {
		return nil, errors.New("t1IDs and t2IDs can not be empty at the same time")
	}

	mergedParams := make(SearchParams)
	for _, param := range params {
		maps.Insert(mergedParams, maps.All(param))
	}

	if len(t1IDs) > 0 {
		mergedParams["in_"+d.jsonFieldName1] = IDsJoin(t1IDs, ",")
	}
	if len(t2IDs) > 0 {
		mergedParams["in_"+d.jsonFieldName2] = IDsJoin(t2IDs, ",")
	}

	u, err := url.Parse(d.baseURL + "/all")
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(mergedParams)
	if err != nil {
		return nil, err
	}

	res := new(R[[]M2MConnector])

	err = MakeJSONRequest[[]M2MConnector](d.httpClient, d.okayHttpStatusRange, u, http.MethodPost, bytes.NewReader(body), res)
	if err != nil {
		return nil, err
	} else if res == nil {
		return nil, errors.New("response is nil")
	}

	return res.Data, nil
}

func (d *M2MConnectorHandler[M1, M2, M2MConnector]) Save(records []M2MConnector) (int64, error) {
	u, err := url.Parse(d.baseURL + "/save")
	if err != nil {
		return -1, err
	}

	body, err := json.Marshal(records)
	if err != nil {
		return -1, err
	}

	res := new(R[int64])
	err = MakeJSONRequest(d.httpClient, d.okayHttpStatusRange, u, http.MethodPut, bytes.NewReader(body), res)
	if err != nil {
		return -1, err
	}

	return res.Data, nil
}

func (d *M2MConnectorHandler[M1, M2, M2MConnector]) SaveAfterDelete(deleteByField string, idToDelete ID, records []M2MConnector) (int64, error) {
	if deleteByField != d.ObjectFieldName1 && deleteByField != d.ObjectFieldName2 {
		return -1, fmt.Errorf("deleteByField must be %s or %s", d.ObjectFieldName1, d.ObjectFieldName2)
	}

	jsonField := ""
	switch deleteByField {
	case d.ObjectFieldName1:
		jsonField = d.jsonFieldName1
	case d.ObjectFieldName2:
		jsonField = d.jsonFieldName2
	}

	u, err := url.Parse(fmt.Sprintf("%s/save/%s/%d", d.baseURL, jsonField, idToDelete))
	if err != nil {
		return -1, err
	}

	for index, record := range records {
		reflected := reflect.ValueOf(record)
		idField := reflected.FieldByName(deleteByField)

		if idField.Uint() != uint64(idToDelete) {
			return -1, fmt.Errorf("%s must be %d at index of %d", deleteByField, idToDelete, index)
		}
	}

	body, err := json.Marshal(records)
	if err != nil {
		return -1, err
	}

	res := new(R[int64])

	err = MakeJSONRequest(d.httpClient, d.okayHttpStatusRange, u, http.MethodPost, bytes.NewReader(body), res)
	if err != nil {
		return -1, err
	} else if res == nil {
		return -1, errors.New("response is nil")
	}

	return res.Data, nil
}

func (d *M2MConnectorHandler[M1, M2, M2MConnector]) Delete(id1, id2 ID) (int64, error) {
	u, err := url.Parse(fmt.Sprintf("%s?%s=%d&%s=%d", d.baseURL, url.QueryEscape(d.jsonFieldName1), id1, url.QueryEscape(d.jsonFieldName2), id2))
	if err != nil {
		return -1, err
	}

	res := new(R[int64])
	err = MakeJSONRequest(d.httpClient, d.okayHttpStatusRange, u, http.MethodDelete, bytes.NewReader(nil), res)
	if err != nil {
		return -1, err
	} else if res == nil {
		return -1, errors.New("response is nil")
	}

	return res.Data, nil
}

func NewM2MConnectorHandler[M1 any, M2 any, M2MConnector any](
	baseURL string,
	httpClient *http.Client,
	okayHttpStatusRange *HttpStatusRange,
	objectFieldName1, objectFieldName2 string,
) (*M2MConnectorHandler[M1, M2, M2MConnector], error) {
	if objectFieldName1 == "" {
		return nil, fmt.Errorf("objectFieldName1 is empty")
	}
	if objectFieldName2 == "" {
		return nil, fmt.Errorf("objectFieldName2 is empty")
	}

	{
		reflected := reflect.TypeOf(new(M2MConnector)).Elem()

		field1, ok := reflected.FieldByName(objectFieldName1)
		if !ok {
			return nil, fmt.Errorf("field %s does NOT exist in %s", objectFieldName1, reflected.Name())
		} else if field1.Type.Kind() != IDKind {
			return nil, fmt.Errorf("field %s type MUST be %s, but got %s", objectFieldName1, IDKind, field1.Type.Kind())
		}

		field2, ok := reflected.FieldByName(objectFieldName2)
		if !ok {
			return nil, fmt.Errorf("field %s does NOT exist in %s", objectFieldName2, reflected.Name())
		} else if field2.Type.Kind() != IDKind {
			return nil, fmt.Errorf("field %s type MUST be %s, but got %s", objectFieldName2, IDKind, field2.Type.Kind())
		}
	}

	jsonFields, err := GetJSONFieldNameOf[M2MConnector](objectFieldName1, objectFieldName2)
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

	handler := &M2MConnectorHandler[M1, M2, M2MConnector]{
		baseURL:             baseURL,
		httpClient:          httpClient,
		okayHttpStatusRange: okayHttpStatusRange,
		ObjectFieldName1:    objectFieldName1,
		ObjectFieldName2:    objectFieldName2,

		jsonFieldName1: jsonFields[0],
		jsonFieldName2: jsonFields[1],
	}
	return handler, nil
}
