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

func RecoveryHandler(responseFullError bool) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, err any) {
		message := "Internal Server PanicError"

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

		c.AbortWithStatusJSON(http.StatusInternalServerError, R[any]{
			Code:    "500",
			Message: message,
			Data:    Ternary(responseFullError, err, nil),
		})
	})
}
