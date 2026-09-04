package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// pathUint parses a numeric path param, writing 400 on garbage.
func pathUint(c *gin.Context, name string) (uint, bool) {
	n, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID tidak valid."})
		return 0, false
	}
	return uint(n), true
}

// queryID parses an optional numeric query filter.
// Returns 0 when absent. When present but invalid, returns max uint so the
// filter matches nothing (same visible result as before with bad input).
func queryID(q func(string) string, name string) uint {
	v := q(name)
	if v == "" {
		return 0
	}
	n, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return ^uint(0)
	}
	return uint(n)
}
