package gocrud

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

type R[T any] struct {
	Code    string `json:"c"`
	Message string `json:"m"`
	Data    T      `json:"d"`
}

func MakeErrorResponse(context *gin.Context, statusCode int, code string, err any) {
	message := "Internal Server Error"

	if err != nil {
		switch err.(type) {
		case string:
			message = ValuableString(Pointer(err.(string)), message)
		case *string:
			message = ValuableString(err.(*string), message)
		case error:
			message = ValuableString(Pointer(err.(error).Error()), message)
		}
	}

	context.AbortWithStatusJSON(statusCode, R[any]{
		Code:    Ternary(code == "", "500", code),
		Message: message,
		Data:    err,
	})
}

func RecoveryHandler(responseFullError bool) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, err any) {
		MakeErrorResponse(
			c,
			http.StatusInternalServerError,
			"500",
			Ternary(responseFullError, err, nil),
		)
	})
}
