package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func ginJSON(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, data)
}

func ginAbort(c *gin.Context, code int, msg string) {
	c.AbortWithStatusJSON(code, gin.H{"error": msg})
}

func parseInt(s string, def int) int {
	i, err := strconv.Atoi(s)
	if err != nil || i <= 0 {
		return def
	}
	return i
}
