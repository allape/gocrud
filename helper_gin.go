package gocrud

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

var RestCoder = NewDefaultCoder()

type R[T any] struct {
	Code    Code   `json:"c"`
	Message string `json:"m,omitempty"`
	Data    T      `json:"d"`
}

func MakeErrorResponse(context *gin.Context, code Code, err any) {
	message := http.StatusText(http.StatusInternalServerError)

	if err != nil {
		switch err.(type) {
		case string:
			if err != "" {
				message = err.(string)
			}
		case *string:
			if err == nil && len(*err.(*string)) > 0 {
				message = *err.(*string)
			}
		case error:
			msg := err.(error).Error()
			if msg != "" {
				message = msg
			}
		}
	}

	context.AbortWithStatusJSON(http.StatusOK, R[any]{
		Code:    Ternary(code == "", RestCoder.InternalServerError(), code),
		Message: message,
	})
}

func MakeOkayResponse[T any](context *gin.Context, code Code, message string, data T) {
	context.JSON(http.StatusOK, R[T]{
		Code:    code,
		Message: message,
		Data:    data,
	})
}

func MakeOkayDataResponse[T any](context *gin.Context, data T) {
	MakeOkayResponse[T](context, RestCoder.OK(), "", data)
}

func RecoveryHandler(responseFullError bool) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, err any) {
		MakeErrorResponse(
			c,
			RestCoder.InternalServerError(),
			Ternary(responseFullError, err, nil),
		)
	})
}
