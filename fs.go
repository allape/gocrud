package gocrud

import (
	"errors"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"os"
	"path"
)

type HttpFileSystemConfig struct {
	AllowUpload    bool
	AllowOverwrite bool
	Coder          Coder
}

func NewHttpFileSystem(group *gin.RouterGroup, folder string, config *HttpFileSystemConfig) error {
	if config == nil {
		config = &HttpFileSystemConfig{Coder: NewDefaultCoder()}
	}

	if config.Coder == nil {
		config.Coder = NewDefaultCoder()
	}

	group.Static("", folder)

	group.POST("/*filepath", func(context *gin.Context) {
		if !config.AllowUpload {
			MakeErrorResponse(context, config.Coder.MethodNotAllowed(), errors.New("upload not allowed"))
			return
		}

		relativeFileName := context.Param("filepath")
		fullFileName := path.Join(folder, relativeFileName)
		baseFolder := path.Dir(fullFileName)

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

		stat, err := os.Stat(fullFileName)
		if err == nil {
			if !config.AllowOverwrite {
				MakeErrorResponse(context, config.Coder.Conflict(), errors.New("file already exists"))
				return
			}
			if stat.IsDir() {
				MakeErrorResponse(context, config.Coder.InternalServerError(), errors.New("file is a directory"))
				return
			}
		} else {
			if !os.IsNotExist(err) {
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

		n, err := io.Copy(file, context.Request.Body)
		if err != nil {
			_ = os.Remove(fullFileName)
			MakeErrorResponse(context, config.Coder.InternalServerError(), err)
			return
		}

		contextLength := context.Request.ContentLength
		if contextLength > 0 && n != contextLength {
			_ = os.Remove(fullFileName)
			MakeErrorResponse(context, config.Coder.InternalServerError(), errors.New("incomplete write"))
			return
		}

		context.JSON(http.StatusOK, R[any]{
			Code: config.Coder.OK(),
			Data: relativeFileName,
		})
	})

	return nil
}
