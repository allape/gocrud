package gocrud

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
)

const (
	XFileDigest = "X-File-Digest"
)

var (
	ErrorIncompleteWrite    = errors.New("incomplete write")
	ErrorFileExists         = errors.New("file already exists")
	ErrorFileIsDir          = errors.New("file is a directory")
	ErrorUploadNotAllowed   = errors.New("upload not allowed")
	ErrorFileDigestMismatch = errors.New("digest mismatch")
)

type HttpFileSystemConfig struct {
	AllowUpload        bool
	AllowOverwrite     bool
	EnableServerDigest bool // EnableServerDigest: if true, will save file with its digest, and discard client defined filename
	Coder              Coder
}

func NewHttpFileSystem(group *gin.RouterGroup, folder string, config *HttpFileSystemConfig) error {
	if config == nil {
		config = &HttpFileSystemConfig{Coder: RestCoder}
	}

	if config.Coder == nil {
		config.Coder = RestCoder
	}

	group.Static("", folder)

	group.POST("/*filepath", func(context *gin.Context) {
		if !config.AllowUpload {
			MakeErrorResponse(context, config.Coder.MethodNotAllowed(), ErrorUploadNotAllowed)
			return
		}

		var reader io.Reader
		var relativeFileName string
		if config.EnableServerDigest {
			tmpFile, err := os.CreateTemp(os.TempDir(), "gocrud-static-*.bin")
			if err != nil {
				MakeErrorResponse(context, config.Coder.InternalServerError(), err)
				return
			}
			defer func() {
				_ = tmpFile.Close()
				_ = os.Remove(tmpFile.Name())
			}()

			hasher := sha256.New()

			mw := io.MultiWriter(tmpFile, hasher)

			n, err := io.Copy(mw, context.Request.Body)
			if err != nil {
				MakeErrorResponse(context, config.Coder.InternalServerError(), err)
				return
			} else if context.Request.ContentLength > 0 && n != context.Request.ContentLength {
				MakeErrorResponse(context, config.Coder.InternalServerError(), ErrorIncompleteWrite)
				return
			}

			digest := hex.EncodeToString(hasher.Sum(nil))

			digestFromClient := context.GetHeader(XFileDigest)
			if digestFromClient != "" && strings.ToLower(digestFromClient) != digest {
				MakeErrorResponse(context, config.Coder.BadRequest(), ErrorFileDigestMismatch)
				return
			}

			_, err = tmpFile.Seek(0, io.SeekStart)
			if err != nil {
				MakeErrorResponse(context, config.Coder.InternalServerError(), err)
				return
			}

			reader = tmpFile
			relativeFileName = path.Join(
				"/",
				digest[:2],
				digest[2:4],
				digest+path.Ext(context.Param("filepath")),
			)
		} else {
			relativeFileName = context.Param("filepath")
			reader = context.Request.Body
		}

		fullFileName := path.Join(folder, relativeFileName)
		baseFolder := path.Dir(fullFileName)

		stat, err := os.Stat(fullFileName)
		if err == nil {
			if !config.AllowOverwrite {
				//MakeErrorResponse(context, config.Coder.Conflict(), ErrorFileExists)
				context.JSON(http.StatusOK, R[any]{
					Code:    config.Coder.OK(),
					Message: ErrorFileExists.Error(),
					Data:    relativeFileName,
				})
				return
			}
			if stat.IsDir() {
				MakeErrorResponse(context, config.Coder.InternalServerError(), ErrorFileIsDir)
				return
			}
		} else {
			if !os.IsNotExist(err) {
				MakeErrorResponse(context, config.Coder.InternalServerError(), err)
				return
			}
		}

		if _, err := os.Stat(baseFolder); err != nil {
			if os.IsNotExist(err) {
				err = os.MkdirAll(baseFolder, os.ModePerm)
				if err != nil {
					MakeErrorResponse(context, config.Coder.InternalServerError(), err)
					return
				}
			} else {
				MakeErrorResponse(context, config.Coder.InternalServerError(), err)
				return
			}
		}

		file, err := os.Create(fullFileName)
		if err != nil {
			MakeErrorResponse(context, config.Coder.InternalServerError(), err)
			return
		}
		defer func() {
			_ = file.Close()
		}()

		n, err := io.Copy(file, reader)
		if err != nil {
			_ = os.Remove(fullFileName)
			MakeErrorResponse(context, config.Coder.InternalServerError(), err)
			return
		}

		contextLength := context.Request.ContentLength
		if contextLength > 0 && n != contextLength {
			_ = os.Remove(fullFileName)
			MakeErrorResponse(context, config.Coder.InternalServerError(), ErrorIncompleteWrite)
			return
		}

		context.JSON(http.StatusOK, R[any]{
			Code: config.Coder.OK(),
			Data: relativeFileName,
		})
	})

	return nil
}
