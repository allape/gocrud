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
	"strings"

	"github.com/allape/gogger"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ContextKeyHandledKeywordIn
// used to mark if inFieldName1 or inFieldName2 has been triggered
const ContextKeyHandledKeywordIn = "gocrud:m2mc:didkeywordin"

type M2MHelper[T any] struct {
	db     *gorm.DB
	logger *gogger.Logger

	ObjectFieldName1   string
	ObjectFieldName2   string
	JSONFieldName1     string
	JSONFieldName2     string
	DatabaseFieldName1 string
	DatabaseFieldName2 string
	InSearchFieldName1 string
	InSearchFieldName2 string

	SearchHandlers SearchHandlers
}

func (m *M2MHelper[T]) ExtractIDsFromRecord(record *T) (id1, id2 ID, err error) {
	reflected := reflect.ValueOf(record).Elem()
	id1 = ID(reflected.FieldByName(m.ObjectFieldName1).Uint())
	id2 = ID(reflected.FieldByName(m.ObjectFieldName2).Uint())
	return
}

// CheckIfInSearchHandled
//
// Use context.IsAborted() to control logic flow
func (m *M2MHelper[T]) CheckIfInSearchHandled(context *gin.Context) {
	_, ok := context.Get(ContextKeyHandledKeywordIn)
	if !ok {
		MakeErrorResponse(context, RestCoder.BadRequest(), fmt.Sprintf("at least one of %s or %s should not be empty", m.InSearchFieldName1, m.InSearchFieldName2))
		return
	}
}

func (m *M2MHelper[T]) GetIDsFromQuery(context *gin.Context) (id1, id2 ID, err error) {
	id1, err = ParseID(context.Query(m.JSONFieldName1))
	if err != nil {
		err = fmt.Errorf("%s is invalid", m.JSONFieldName1)
		return
	}
	id2, err = ParseID(context.Query(m.JSONFieldName2))
	if err != nil {
		err = fmt.Errorf("%s is invalid", m.JSONFieldName2)
		return
	}
	return
}

func (m *M2MHelper[T]) OnDelete(context *gin.Context) {
	id1, id2, err := m.GetIDsFromQuery(context)
	if err != nil {
		MakeErrorResponse(context, RestCoder.BadRequest(), err)
		return
	}

	res := m.db.Delete(new(T), fmt.Sprintf("`%s` = ? AND `%s` = ?", m.DatabaseFieldName1, m.DatabaseFieldName2), id1, id2)
	if res.Error != nil {
		m.logger.Error().Printf("failed to delete at %d,%d: %v", id1, id2, res.Error)
		MakeErrorResponse(context, RestCoder.InternalServerError(), "[error] failed to delete")
		return
	}

	MakeOkayDataResponse(context, res.RowsAffected > 0)
}

func NewM2MHelper[T any](db *gorm.DB, logger *gogger.Logger, objectFieldName1, objectFieldName2 string) (*M2MHelper[T], error) {
	reflected := reflect.TypeFor[T]()

	// Semantic Check: this op will not panic but emit warnings
	{
		const EXAMPLE = "\nFor example:\n" +
			"T: UserPermission, objectFieldName1: UserID,    objectFieldName2: PermissionID\n" +
			"T: GalleryPhoto,   objectFieldName1: GalleryID, objectFieldName2: PhotoID\n" +
			"T: MusicArtist,    objectFieldName1: MusicID,   objectFieldName2: ArtistID\n" +
			"And the order is essential too."

		lowerCasedField1 := strings.ToLower(objectFieldName1)
		if !strings.HasSuffix(lowerCasedField1, "id") {
			logger.Warn().Printf("Semantic Check: objectFieldName1 %s should end with 'id'.\n%s", objectFieldName1, EXAMPLE)
		}

		lowerCasedField2 := strings.ToLower(objectFieldName2)
		if !strings.HasSuffix(lowerCasedField2, "id") {
			logger.Warn().Printf("Semantic Check: objectFieldName2 %s should end with 'id'.\n%s", objectFieldName2, EXAMPLE)
		}

		lowerCasedTName := strings.ToLower(reflected.Name())
		lowerCasedMerged := strings.TrimSuffix(lowerCasedField1, "id") + strings.TrimSuffix(lowerCasedField2, "id")
		if !strings.HasPrefix(lowerCasedTName, lowerCasedMerged) {
			logger.Warn().Printf(
				"Semantic Check: T name %s should start with %s.\n%s",
				reflected.Name(),
				lowerCasedMerged,
				EXAMPLE,
			)
		}
	}

	ofn1, ok := reflected.FieldByName(objectFieldName1)
	if !ok {

		return nil, fmt.Errorf("field %s is invalid", objectFieldName1)
	} else if ofn1.Type.Kind() != IDKind {

		return nil, fmt.Errorf("type of field %s is invalid, should be %d", objectFieldName1, IDKind)
	}

	ofn2, ok := reflected.FieldByName(objectFieldName2)
	if !ok {
		return nil, fmt.Errorf("field %s is invalid", objectFieldName2)
	} else if ofn2.Type.Kind() != IDKind {
		return nil, fmt.Errorf("type of field %s is invalid, should be %d", objectFieldName2, IDKind)
	}

	jsonFields, err := GetJSONFieldNameOf[T](objectFieldName1, objectFieldName2)
	if err != nil || len(jsonFields) != 2 {

		return nil, fmt.Errorf("failed to get JSON fields: %v", err)
	}

	jsonFieldName1 := jsonFields[0]
	jsonFieldName2 := jsonFields[1]

	databaseFields, err := GetDatabaseFieldNameOf[T](db, objectFieldName1, objectFieldName2)
	if err != nil || len(databaseFields) != 2 {
		return nil, fmt.Errorf("failed to get database fields: %v", err)
	}

	databaseFieldName1 := databaseFields[0]
	databaseFieldName2 := databaseFields[1]

	inSearchFieldName1 := "in_" + jsonFieldName1
	inSearchFieldName2 := "in_" + jsonFieldName2

	var handleKeywordIdIn = func(databaseFieldName string) SearchHandler {
		return func(db *gorm.DB, values []string, context *gin.Context) (*gorm.DB, error) {
			return KeywordIDIn(databaseFieldName, func(value []ID) []ID {
				if len(value) > 0 {
					context.Set(ContextKeyHandledKeywordIn, true)
				}
				return value
			})(db, values, context)
		}
	}

	searchHandlers := SearchHandlers{
		inSearchFieldName1: handleKeywordIdIn(databaseFieldName1),
		inSearchFieldName2: handleKeywordIdIn(databaseFieldName2),
	}

	return &M2MHelper[T]{
		db:     db,
		logger: logger,

		ObjectFieldName1:   objectFieldName1,
		ObjectFieldName2:   objectFieldName2,
		JSONFieldName1:     jsonFieldName1,
		JSONFieldName2:     jsonFieldName2,
		DatabaseFieldName1: databaseFieldName1,
		DatabaseFieldName2: databaseFieldName2,
		InSearchFieldName1: inSearchFieldName1,
		InSearchFieldName2: inSearchFieldName2,

		SearchHandlers: searchHandlers,
	}, nil
}

type SetupM2MConnectorControllerOptions[T any] struct {
	// OnRecordCheck
	// use context to abort process
	OnRecordCheck func(record *T, db *gorm.DB, context *gin.Context)

	// OnDelete
	// use this to replace the original delete operation
	OnDelete func(db *gorm.DB, context *gin.Context)

	ExtraSearchHandlers SearchHandlers
}

// SetupM2MConnectorController
// M2M: Many to Many, Models to Models
func SetupM2MConnectorController[T any](
	group *gin.RouterGroup, db *gorm.DB, logger *gogger.Logger,
	objectFieldName1, objectFieldName2 string,
	options *SetupM2MConnectorControllerOptions[T],
) error {
	if objectFieldName1 == "" || objectFieldName2 == "" {
		return fmt.Errorf("field1 and field2 cannot be empty")
	}

	if options == nil {
		options = &SetupM2MConnectorControllerOptions[T]{}
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

// SetupM2MCrud
//
// Warning:
//
// [crud.Coder] is not used in this spec
//
// [crud.OnGetOne] will be overwritten
//
// [crud.OnDelete] will be overwritten
func SetupM2MCrud[T any](
	group *gin.RouterGroup, db *gorm.DB, logger *gogger.Logger,
	objectFieldName1, objectFieldName2 string,
	crud *Crud[T],
) error {
	if crud == nil {
		crud = &Crud[T]{}
	}

	m2mHelper, err := NewM2MHelper[T](db, logger, objectFieldName1, objectFieldName2)
	if err != nil {
		return err
	}

	crud.SearchHandlers = MergeSearchHandlers(m2mHelper.SearchHandlers, crud.SearchHandlers)

	oldBeforeGetAll := crud.BeforeGetAll
	crud.BeforeGetAll = func(context *gin.Context, db *gorm.DB) *gorm.DB {
		m2mHelper.CheckIfInSearchHandled(context)
		if context.IsAborted() {
			return db
		}

		if oldBeforeGetAll != nil {
			return oldBeforeGetAll(context, db)
		}

		return db
	}

	oldBeforeSave := crud.BeforeSave
	crud.BeforeSave = func(record *T, context *gin.Context, db *gorm.DB) {
		id1, id2, err := m2mHelper.ExtractIDsFromRecord(record)
		if err != nil {
			MakeErrorResponse(context, RestCoder.BadRequest(), "[error] failed to extract ids")
			return
		}
		if id1 == 0 {
			MakeErrorResponse(context, RestCoder.BadRequest(), fmt.Sprintf("%s can not be 0", m2mHelper.JSONFieldName1))
			return
		}
		if id2 == 0 {
			MakeErrorResponse(context, RestCoder.BadRequest(), fmt.Sprintf("%s can not be 0", m2mHelper.JSONFieldName2))
			return
		}

		if oldBeforeSave != nil {
			oldBeforeSave(record, context, db)
		}
	}

	crud.OnGetOne = func(context *gin.Context, db *gorm.DB) *gorm.DB {
		id1, id2, err := m2mHelper.GetIDsFromQuery(context)
		if err != nil {
			MakeErrorResponse(context, RestCoder.BadRequest(), err)
			return db
		}

		return db.Where(fmt.Sprintf("`%s` = ? AND `%s` = ?", m2mHelper.DatabaseFieldName1, m2mHelper.DatabaseFieldName2), id1, id2)
	}

	crud.OnDelete = func(context *gin.Context, db *gorm.DB) bool {
		m2mHelper.OnDelete(context)
		return true
	}

	return Setup[T](group, db, logger, crud)
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
	}
	return res.Data, nil
}

func (d *M2MConnectorHandler[M1, M2, M2MConnector]) Delete(id1, id2 ID) (bool, error) {
	u, err := url.Parse(fmt.Sprintf("%s?%s=%d&%s=%d", d.baseURL, url.QueryEscape(d.jsonFieldName1), id1, url.QueryEscape(d.jsonFieldName2), id2))
	if err != nil {
		return false, err
	}

	res := new(R[bool])
	err = MakeJSONRequest(d.httpClient, d.okayHttpStatusRange, u, http.MethodDelete, bytes.NewReader(nil), res)
	if err != nil {
		return false, err
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
		reflected := reflect.TypeFor[M2MConnector]()

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
