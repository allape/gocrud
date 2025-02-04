package gocrud

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

var RestCoder = NewDefaultCoder()

type R[T any] struct {
	Code    Code   `json:"c"`
	Message string `json:"m"`
	Data    T      `json:"d"`
}

func MakeErrorResponse(context *gin.Context, code Code, err any) {
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

	context.AbortWithStatusJSON(http.StatusOK, R[any]{
		Code:    Ternary(code == "", RestCoder.InternalServerError(), code),
		Message: message,
		Data:    err,
	})
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

func Ternary[T any](condition bool, onTrue T, onFalse T) T {
	if condition {
		return onTrue
	}
	return onFalse
}

func Pick[T any](arr []T, index int, defaultValue T) T {
	if index < 0 || index >= len(arr) {
		return defaultValue
	}
	return arr[index]
}

func ValuableString(str *string, ifEmptyValue string) string {
	if str == nil || *str == "" {
		return ifEmptyValue
	}
	return *str
}

func ValuableArray(array []string) (bool, string) {
	if len(array) > 0 && array[0] != "" {
		return true, array[0]
	}
	return false, ""
}

func Pointer[T any](t T) *T {
	return &t
}

func NowString(pattern *string) string {
	return time.Now().Format(ValuableString(pattern, "2006-01-02 15:04:05.000"))
}
