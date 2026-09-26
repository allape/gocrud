package gocrud

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func DefaultCorsConfig() cors.Config {
	config := cors.DefaultConfig()
	config.AddAllowHeaders(XFileDigest)
	config.AllowAllOrigins = true
	return config
}

func NewCors() gin.HandlerFunc {
	return cors.New(DefaultCorsConfig())
}
