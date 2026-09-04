package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/0xMinomus/openPOS/backend/service"
)

const claimsKey = "claims"

func Auth(authSvc *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(401, gin.H{"error": "token tidak disertakan"})
			return
		}
		claims, err := authSvc.ParseAccess(strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "sesi tidak valid, silakan masuk kembali"})
			return
		}
		c.Set(claimsKey, claims)
		c.Next()
	}
}

func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := ClaimsFrom(c)
		if claims == nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "sesi tidak valid"})
			return
		}
		for _, role := range roles {
			if string(claims.Role) == role {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(403, gin.H{"error": "akses ditolak untuk peran Anda"})
	}
}

func ClaimsFrom(c *gin.Context) *service.Claims {
	v, exists := c.Get(claimsKey)
	if !exists {
		return nil
	}
	claims, _ := v.(*service.Claims)
	return claims
}
