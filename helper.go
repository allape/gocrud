package gocrud

import (
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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

func Ternary[T any](condition bool, onTrue T, onFalse T) T {
	if condition {
		return onTrue
	}
	return onFalse
}

func TernaryFunc[T any](conditionFunc func() bool, onTrueFunc, onFalseFunc func() T) T {
	if conditionFunc() {
		return onTrueFunc()
	}
	return onFalseFunc()
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

func PickFirstValuableString(array []string) (string, bool) {
	if len(array) > 0 && array[0] != "" {
		return array[0], true
	}
	return "", false
}

func Pointer[T any](t T) *T {
	return &t
}

func NowString(pattern *string) string {
	return time.Now().Format(ValuableString(pattern, "2006-01-02 15:04:05.000"))
}

func MapFuncOverCommaSeparatedString(mapFunc func(string), css string) {
	for _, s := range strings.Split(css, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		mapFunc(s)
	}
}

func StringArrayFromCommaSeparatedString(css string) []string {
	var array []string
	MapFuncOverCommaSeparatedString(func(s string) {
		array = append(array, s)
	}, css)
	return array
}

func RemoveDuplication[T ~[]E, E comparable](array T) T {
	fresh := make(T, 0, len(array))
	for _, v := range array {
		if slices.Contains(fresh, v) {
			continue
		}
		fresh = append(fresh, v)
	}
	return fresh
}
