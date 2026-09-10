package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/adrr-dev/openPOS/backend/service"
)

func ActiveCheck(authSvc *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := ClaimsFrom(c)
		if claims == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "sesi tidak valid"})
			return
		}
		// Reuse Me logic to validate active status (checks user/cashier Active)
		if _, err := authSvc.Me(c.Request.Context(), claims); err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "akun dinonaktifkan"})
			return
		}
		c.Next()
	}
}
