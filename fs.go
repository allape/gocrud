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
}

func NewPostFileHandler(folder string, config HttpFileSystemConfig) gin.HandlerFunc {
	return func(context *gin.Context) {
		relativeFileName := context.Param("filepath")
		fullFileName := path.Join(folder, relativeFileName)
		baseFolder := path.Dir(fullFileName)

		if _, err := os.Stat(baseFolder); err != nil {
			if os.IsNotExist(err) {
				err = os.MkdirAll(baseFolder, os.ModePerm)
				if err != nil {
					MakeErrorResponse(context, http.StatusInternalServerError, "500", err)
					return
				}
			} else {
				MakeErrorResponse(context, http.StatusInternalServerError, "500", err)
				return
			}
		}

		stat, err := os.Stat(fullFileName)
		if err == nil {
			if !config.AllowOverwrite {
				MakeErrorResponse(context, http.StatusConflict, "409", errors.New("file already exists"))
				return
			}
			if stat.IsDir() {
				MakeErrorResponse(context, http.StatusInternalServerError, "500", errors.New("file is a directory"))
				return
			}
		} else {
			if !os.IsNotExist(err) {
				MakeErrorResponse(context, http.StatusInternalServerError, "500", err)
				return
			}
		}

		file, err := os.Create(fullFileName)
		if err != nil {
			MakeErrorResponse(context, http.StatusInternalServerError, "500", err)
			return
		}
		defer func() {
			_ = file.Close()
		}()

		n, err := io.Copy(file, context.Request.Body)
		if err != nil {
			_ = os.Remove(fullFileName)
			MakeErrorResponse(context, http.StatusInternalServerError, "500", err)
			return
		}

		contextLength := context.Request.ContentLength
		if contextLength > 0 && n != contextLength {
			_ = os.Remove(fullFileName)
			MakeErrorResponse(context, http.StatusInternalServerError, "500", errors.New("incomplete write"))
			return
		}

		context.JSON(http.StatusOK, R[any]{
			Code:    "0",
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
