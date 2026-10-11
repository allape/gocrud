package gocrud

import (
	"fmt"

	"github.com/allape/gogger"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

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
		// do NOT check this in crud, use controller to limit this
		//m2mHelper.CheckIfInSearchHandled(context)
		//if context.IsAborted() {
		//	return db
		//}

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
