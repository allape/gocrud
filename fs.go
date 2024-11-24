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
	AllowOverwrite bool
	ErrorCoder     Coder
}

func NewPostFileHandler(folder string, config HttpFileSystemConfig) gin.HandlerFunc {
	if config.ErrorCoder == nil {
		config.ErrorCoder = &DefaultCoder{}
	}
	return func(context *gin.Context) {
		relativeFileName := context.Param("filepath")
		fullFileName := path.Join(folder, relativeFileName)
		baseFolder := path.Dir(fullFileName)

		if _, err := os.Stat(baseFolder); err != nil {
			if os.IsNotExist(err) {
				err = os.MkdirAll(baseFolder, os.ModePerm)
				if err != nil {
					MakeErrorResponse(context, config.ErrorCoder.InternalServerError(), err)
					return
				}
			} else {
				MakeErrorResponse(context, config.ErrorCoder.InternalServerError(), err)
				return
			}
		}

		stat, err := os.Stat(fullFileName)
		if err == nil {
			if !config.AllowOverwrite {
				MakeErrorResponse(context, config.ErrorCoder.Conflict(), errors.New("file already exists"))
				return
			}
			if stat.IsDir() {
				MakeErrorResponse(context, config.ErrorCoder.InternalServerError(), errors.New("file is a directory"))
				return
			}
		} else {
			if !os.IsNotExist(err) {
				MakeErrorResponse(context, config.ErrorCoder.InternalServerError(), err)
				return
			}
		}

		file, err := os.Create(fullFileName)
		if err != nil {
			MakeErrorResponse(context, config.ErrorCoder.InternalServerError(), err)
			return
		}
		defer func() {
			_ = file.Close()
		}()

		n, err := io.Copy(file, context.Request.Body)
		if err != nil {
			_ = os.Remove(fullFileName)
			MakeErrorResponse(context, config.ErrorCoder.InternalServerError(), err)
			return
		}

		contextLength := context.Request.ContentLength
		if contextLength > 0 && n != contextLength {
			_ = os.Remove(fullFileName)
			MakeErrorResponse(context, config.ErrorCoder.InternalServerError(), errors.New("incomplete write"))
			return
		}

		context.JSON(http.StatusOK, R[any]{
			Code:    config.ErrorCoder.OK(),
			Message: "OK",
			Data:    relativeFileName,
		})
	}
}

func NewHttpFileSystem(group *gin.RouterGroup, folder string, config HttpFileSystemConfig) error {
	group.POST("/*filepath", NewPostFileHandler(folder, config))
	group.Static("", folder)
	return nil
}
