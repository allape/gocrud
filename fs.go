package gocrud

import (
	"errors"
	"net/http"
	"os"
	"path"

	"github.com/gin-gonic/gin"
)

const (
	XFileDigest = "X-File-Digest"
)

var (
	ErrorIncompleteWrite            = errors.New("incomplete write")
	ErrorFileIsDir                  = errors.New("file is a directory")
	ErrorUploadNotAllowed           = errors.New("upload not allowed")
	ErrorFileDigestMismatch         = errors.New("digest mismatch")
	ErrorFileMasterKeyMustBe32Bytes = errors.New("master key must be 32 bytes")
	ErrorFileKeyProviderIsNil       = errors.New("file key provider is nil")
)

type (
	FileName        string
	FileSize        int64
	FileDigest      string
	FileSaltyDigest string

	FileNonce     []byte // Use this to generate FileKey From FileMasterKey
	FileMasterKey []byte // This should be saved securely, all FileKey-s are derived from this
	FileKey       []byte
	FileHashSalt  FileNonce
)

type HttpFile struct {
	Name        FileName
	Size        FileSize
	Nonce       FileNonce
	FileKey     FileKey
	Digest      FileDigest
	SaltyDigest FileSaltyDigest
}

type HttpFileSystemConfig struct {
	AllowUpload bool

	// EnableDigest
	// if true, will save file with its digest, and discard client defined filename
	//
	// DEPRECATED: forced be digested now
	//EnableDigest bool

	FileMasterKey  FileMasterKey
	FileHashSalt   FileHashSalt
	OnFileReview   func(filename FileName) (*HttpFile, error)
	OnFileDigested func(digest FileDigest, saltyDigest FileSaltyDigest) (*HttpFile, error)
	OnFileSaved    func(file *HttpFile) error

	Coder Coder
}

func NewHttpFileSystemController(group *gin.RouterGroup, folder string, config *HttpFileSystemConfig) error {
	if config == nil {
		config = &HttpFileSystemConfig{Coder: RestCoder}
	}

	if config.Coder == nil {
		config.Coder = RestCoder
	}

	if config.FileMasterKey == nil {
		group.Static("", folder)
	} else {
		if config.OnFileReview == nil {
			return ErrorFileKeyProviderIsNil
		}

		group.GET("/*filepath", func(context *gin.Context) {
			httpFile, err := config.OnFileReview(FileName(context.Param("filepath")))
			if err != nil {
				MakeErrorResponse(context, config.Coder.InternalServerError(), err)
				return
			}
			if httpFile == nil {
				MakeErrorResponse(context, config.Coder.NotFound(), http.StatusText(http.StatusNotFound))
				return
			}

			filePath := path.Join(folder, string(httpFile.Name))
			file, err := os.Open(filePath)
			if err != nil {
				if os.IsNotExist(err) {
					MakeErrorResponse(context, config.Coder.NotFound(), http.StatusText(http.StatusNotFound))
					return
				}
				MakeErrorResponse(context, config.Coder.InternalServerError(), err)
				return
			}
			defer func() {
				_ = file.Close()
			}()

			serveFunc, err := NewDareHttpServeFunc(file, httpFile)
			if err != nil {
				MakeErrorResponse(context, config.Coder.InternalServerError(), err)
				return
			}

			serveFunc(context.Writer, context.Request)
		})
	}

	uploadHandler := func(context *gin.Context) {
		if !config.AllowUpload {
			MakeErrorResponse(context, config.Coder.MethodNotAllowed(), ErrorUploadNotAllowed)
			return
		}

		file, err := SaveDareFile(
			context.Request.Body,
			&SaveDareFileConfig{
				BaseFolder:     folder,
				Ext:            path.Ext(path.Base(context.Param("filepath"))),
				Size:           FileSize(context.Request.ContentLength),
				Validigest:     FileDigest(context.GetHeader(XFileDigest)),
				MasterKey:      config.FileMasterKey,
				HashSalt:       config.FileHashSalt,
				OnFileDigested: config.OnFileDigested,
			},
		)
		if err != nil {
			MakeErrorResponse(context, config.Coder.InternalServerError(), err)
			return
		}

		if config.OnFileSaved != nil {
			err := config.OnFileSaved(file)
			if err != nil {
				MakeErrorResponse(context, config.Coder.InternalServerError(), err)
				return
			}
		}

		context.JSON(http.StatusOK, R[string]{
			Code: config.Coder.OK(),
			Data: string(file.Name),
		})
	}

	group.POST("/*filepath", uploadHandler)
	group.PUT("/*filepath", uploadHandler)

	return nil
}
