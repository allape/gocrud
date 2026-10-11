package gocrud

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/allape/gogger"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ContextKeyM2MHandledKeywordIn
// used to mark if inFieldName1 or inFieldName2 has been triggered
const ContextKeyM2MHandledKeywordIn = "gocrud:m2m:didkeywordin"

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
	_, ok := context.Get(ContextKeyM2MHandledKeywordIn)
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
					context.Set(ContextKeyM2MHandledKeywordIn, true)
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
