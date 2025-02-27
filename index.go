package gocrud

import (
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"os"
)

type SingleHTMLServeConfig struct {
	AllowReplace bool
	Coder        Coder
}

func NewSingleHTMLServe(group *gin.RouterGroup, indexHTMLFile string, config *SingleHTMLServeConfig) error {
	if config == nil {
		config = &SingleHTMLServeConfig{}
	}

	coder := config.Coder
	if coder == nil {
		coder = RestCoder
	}

	group.StaticFile("/", indexHTMLFile)
	group.StaticFile("/index", indexHTMLFile)
	group.StaticFile("/index.html", indexHTMLFile)

	putHandler := func(context *gin.Context) {
		if !config.AllowReplace {
			MakeErrorResponse(context, coder.MethodNotAllowed(), fmt.Errorf("replace index.html is not allowed"))
			return
		}

		tmpFile, err := os.CreateTemp(os.TempDir(), "gocrud-index-*.html")
		if err != nil {
			MakeErrorResponse(context, coder.InternalServerError(), err)
			return
		}
		defer func() {
			_ = tmpFile.Close()
			_ = os.Remove(tmpFile.Name())
		}()

		n, err := io.Copy(tmpFile, context.Request.Body)
		if err != nil {
			MakeErrorResponse(context, coder.InternalServerError(), err)
			return
		} else if n != context.Request.ContentLength {
			MakeErrorResponse(
				context,
				coder.BadRequest(),
				fmt.Errorf("expected %d bytes, got %d", context.Request.ContentLength, n),
			)
			return
		}

		file, err := os.Create(indexHTMLFile)
		if err != nil {
			MakeErrorResponse(context, coder.InternalServerError(), err)
			return
		}
		defer func() {
			_ = file.Close()
		}()

		_, err = tmpFile.Seek(0, io.SeekStart)
		if err != nil {
			MakeErrorResponse(context, coder.InternalServerError(), err)
			return
		}

		_, err = io.Copy(file, tmpFile)
		if err != nil {
			MakeErrorResponse(context, coder.InternalServerError(), err)
			return
		}

		context.JSON(http.StatusOK, R[any]{
			Code: coder.OK(),
		})
	}

	group.PUT("/", putHandler)
	group.PUT("/index", putHandler)
	group.PUT("/index.html", putHandler)

	return nil
}

func DefaultCorsConfig() cors.Config {
	config := cors.DefaultConfig()
	config.AddAllowHeaders(XFileDigest)
	config.AllowAllOrigins = true
	return config
}

func NewCors() gin.HandlerFunc {
	return cors.New(DefaultCorsConfig())
}
