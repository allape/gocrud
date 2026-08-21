package gocrud

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/allape/gogger"
	"github.com/allape/gosalty"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type HttpFileSystemObjectBase struct {
	HttpFile

	Filename     Filename         `json:"filename"`
	Length       FileSize         `json:"length"`
	Digest       FileDigest       `json:"digest" gorm:"-"` // This hash should not be saved
	Nonce        FileNonce        `json:"nonce"`
	NoncedDigest FileNoncedDigest `json:"noncedDigest" gorm:"primaryKey"`
	FileKey      FileKey          `json:"fileKey"`

	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime;<-:create"`
}

func (obj *HttpFileSystemObjectBase) ToHttpFile() *HttpFile {
	return &HttpFile{
		Filename:     obj.Filename,
		Length:       obj.Length,
		Digest:       FileDigest(obj.NoncedDigest), //Digest:       obj.Digest,
		Nonce:        obj.Nonce,
		NoncedDigest: obj.NoncedDigest,
		FileKey:      obj.FileKey,
	}
}

func (obj *HttpFileSystemObjectBase) FromHttpFile(file *HttpFile) {
	obj.Filename = file.Filename
	obj.Length = file.Length
	//obj.Digest = file.Digest
	obj.Digest = FileDigest(file.NoncedDigest)
	obj.Nonce = file.Nonce
	obj.NoncedDigest = file.NoncedDigest
	obj.FileKey = file.FileKey
}

type HttpFileSystemObjectConfig[T any] struct {
	AllowUpload   bool
	FileMasterKey FileMasterKey
}

// NewHttpFileSystemConfig
// folder: The folder for saving files
// baseStructFieldName: Field name of HttpFileSystemObjectBase inside T, default is "HttpFileSystemObjectBase"
func (h *HttpFileSystemObjectConfig[T]) NewHttpFileSystemConfig(
	db *gorm.DB, logger *gogger.Logger,
	folder string, baseStructFieldName string,
) (*HttpFileSystemConfig, error) {
	baseConfig := h
	if h == nil {
		baseConfig = &HttpFileSystemObjectConfig[T]{}
	}

	config := &HttpFileSystemConfig{
		AllowUpload:   baseConfig.AllowUpload,
		FileMasterKey: baseConfig.FileMasterKey,
	}

	if baseStructFieldName == "" {
		reflected := reflect.TypeFor[HttpFileSystemObjectBase]()
		baseStructFieldName = reflected.Name()
	}

	// precheck
	{
		reflected := reflect.TypeFor[T]()

		baseStructField, ok := reflected.FieldByName(baseStructFieldName)
		if !ok {
			return nil, fmt.Errorf("baseStructField %s not found", baseStructFieldName)
		}

		baseStructFieldType := baseStructField.Type

		if baseStructFieldType.Kind() == reflect.Pointer {
			baseStructFieldType = baseStructFieldType.Elem()
		}

		if baseStructFieldType.Kind() != reflect.Struct {
			return nil, fmt.Errorf("baseStructFieldType %s is not a struct", baseStructFieldType)
		}

		fileSystemObjectBaseType := reflect.TypeFor[HttpFileSystemObjectBase]()

		if !baseStructFieldType.ConvertibleTo(fileSystemObjectBaseType) {
			return nil, fmt.Errorf(
				"baseStructField %s should be a type of %s, but got %s",
				baseStructFieldName,
				fileSystemObjectBaseType.Name(),
				baseStructFieldType.Name(),
			)
		}
	}

	err := db.AutoMigrate(new(T))
	if err != nil {
		return nil, err
	}

	getObjectBase := func(record *T) (*HttpFileSystemObjectBase, error) {
		value := reflect.ValueOf(record).Elem().FieldByName(baseStructFieldName)

		if value.Kind() == reflect.Pointer {
			value = value.Elem()
		}

		obj, ok := value.Interface().(HttpFileSystemObjectBase)
		if !ok {
			return nil, fmt.Errorf("unexpected type %T", record)
		}

		if len(config.FileMasterKey) > 0 && len(obj.FileKey) > 0 && len(obj.Nonce) > 0 {
			decrypted, err := gosalty.SaltyDecode(obj.FileKey, config.FileMasterKey, obj.Nonce)
			if err != nil {
				return nil, err
			}
			obj.FileKey = decrypted
		}

		return &obj, nil
	}

	setObjectBase := func(record *T, obj *HttpFileSystemObjectBase) error {
		if len(config.FileMasterKey) > 0 && len(obj.FileKey) > 0 && len(obj.Nonce) > 0 {
			encrypted, err := gosalty.SaltyEncode(obj.FileKey, config.FileMasterKey, obj.Nonce)
			if err != nil {
				return err
			}
			obj.FileKey = encrypted
		}

		value := reflect.ValueOf(record).Elem().FieldByName(baseStructFieldName)
		if value.Kind() == reflect.Pointer {
			value = value.Elem()
		}
		value.Set(reflect.ValueOf(obj).Elem())
		return nil
	}

	config.OnFileReview = func(filename Filename) (*HttpFile, error) {
		noncedDigest := path.Base(string(filename))
		dotIndex := strings.Index(noncedDigest, ".")
		if dotIndex >= 0 {
			noncedDigest = noncedDigest[:dotIndex]
		}

		if noncedDigest == "" {
			return nil, nil
		}

		var records []T
		if err = db.Model(new(T)).Where("`nonced_digest` = ?", noncedDigest).Find(&records).Error; err != nil {
			logger.Error().Printf("OnFileReview: failed to find file object %s:%s:%s, err: %s", folder, filename, noncedDigest, err)
			return nil, err
		}

		if len(records) == 0 {
			return nil, nil
		}

		obj, err := getObjectBase(&records[0])
		if err != nil {
			return nil, err
		}

		return obj.ToHttpFile(), nil
	}

	config.OnFileDigested = func(_ FileDigest, noncedDigest FileNoncedDigest) (*HttpFile, error) {
		var records []T
		if err = db.Model(new(T)).Where("`nonced_digest` = ?", noncedDigest).Find(&records).Error; err != nil {
			logger.Error().Printf("OnFileDigested: failed to find file object %s:%s, err: %s", folder, noncedDigest, err)
			return nil, err
		}

		if len(records) == 0 {
			return nil, nil
		}

		obj, err := getObjectBase(&records[0])
		if err != nil {
			return nil, err
		}

		return obj.ToHttpFile(), nil
	}

	config.OnFileSaved = func(file *HttpFile) error {
		obj := new(HttpFileSystemObjectBase)
		obj.FromHttpFile(file)

		var record T
		err = setObjectBase(&record, obj)
		if err != nil {
			return err
		}

		if err := db.Model(&record).Save(&record).Error; err != nil {
			return err
		}

		return nil
	}

	return config, nil
}

func NewHttpFileSystemObjectConfig[T any](allowUpload bool, fileMasterKey FileMasterKey) *HttpFileSystemObjectConfig[T] {
	return &HttpFileSystemObjectConfig[T]{
		AllowUpload:   allowUpload,
		FileMasterKey: fileMasterKey,
	}
}

// NewHttpFileSystemObjectController
// [T]: must be extended from HttpFileSystemObjectBase
// baseStructFieldName: will be HttpFileSystemObjectBase if empty
func NewHttpFileSystemObjectController[T any](
	group *gin.RouterGroup, db *gorm.DB, logger *gogger.Logger,
	folder string, baseConfig *HttpFileSystemObjectConfig[T],
	baseStructFieldName string,
) error {
	config, err := baseConfig.NewHttpFileSystemConfig(db, logger, folder, baseStructFieldName)
	if err != nil {
		return err
	}
	return NewHttpFileSystemController(group, folder, config)
}

// region HttpFileSystemObjectHandler

var (
	HttpFileSystemObjectNotFoundError = errors.New("object not found")
)

type HttpFileSystemObjectHandler struct {
	BaseFolder string
	Config     *HttpFileSystemConfig
}

func (h *HttpFileSystemObjectHandler) Open(digestOrFilename string) (*os.File, *HttpFile, error) {
	httpFile, err := h.Config.OnFileReview(Filename(digestOrFilename))
	if err != nil {
		return nil, nil, err
	}
	if httpFile == nil {
		return nil, nil, HttpFileSystemObjectNotFoundError
	}

	filePath := path.Join(h.BaseFolder, string(httpFile.Filename))
	file, err := os.Open(filePath)
	if err != nil {
		return nil, nil, err
	}

	return file, httpFile, nil
}

func (h *HttpFileSystemObjectHandler) NewReader(digestOrFilename string) (*DareReader, error) {
	file, httpFile, err := h.Open(digestOrFilename)
	if err != nil {
		return nil, err
	}

	return NewDareReader(file, httpFile.Length, httpFile.FileKey)
}

func (h *HttpFileSystemObjectHandler) NewServeFunc(digestOrFilename string) (http.HandlerFunc, error) {
	file, httpFile, err := h.Open(digestOrFilename)
	if err != nil {
		return nil, err
	}

	return NewDareHttpServeFunc(file, httpFile)
}

func (h *HttpFileSystemObjectHandler) Save(reader io.Reader, ext string, size FileSize, validigest FileDigest) (*HttpFile, error) {
	file, err := SaveDareFile(
		reader,
		&SaveDareFileConfig{
			BaseFolder:     h.BaseFolder,
			Ext:            ext,
			Length:         size,
			Validigest:     validigest,
			MasterKey:      h.Config.FileMasterKey,
			OnFileDigested: h.Config.OnFileDigested,
		},
	)
	if err != nil {
		return nil, err
	}

	if h.Config.OnFileSaved != nil {
		err := h.Config.OnFileSaved(file)
		if err != nil {
			return nil, err
		}
	}

	return file, nil
}

func NewHttpFileSystemObjectHandler[T any](
	db *gorm.DB, logger *gogger.Logger,
	folder string, masterKey FileMasterKey,
	baseStructFieldName string,
) (*HttpFileSystemObjectHandler, error) {
	baseConfig := NewHttpFileSystemObjectConfig[T](true, masterKey)
	config, err := baseConfig.NewHttpFileSystemConfig(db, logger, folder, baseStructFieldName)
	if err != nil {
		return nil, err
	}

	handler := &HttpFileSystemObjectHandler{
		BaseFolder: folder,
		Config:     config,
	}

	return handler, nil
}

// endregion
