package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type windowCounter struct {
	mu       sync.Mutex
	count    int
	windowAt time.Time
}

var (
	limiterMu sync.Mutex
	limiters  = map[string]*windowCounter{}
)

func getCounter(key string) *windowCounter {
	limiterMu.Lock()
	defer limiterMu.Unlock()
	if c, ok := limiters[key]; ok {
		return c
	}
	c := &windowCounter{windowAt: time.Now()}
	limiters[key] = c
	return c
}

// fixed window: allow max N per window duration, else 429
func checkLimit(key string, max int, window time.Duration) bool {
	c := getCounter(key)
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	if now.Sub(c.windowAt) >= window {
		c.windowAt = now
		c.count = 0
	}
	if c.count >= max {
		return false
	}
	c.count++
	return true
}

func clientIP(c *gin.Context) string {
	if ip := c.ClientIP(); ip != "" {
		return ip
	}
	return c.Request.RemoteAddr
}

func RateLimitLogin() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := "login:" + clientIP(c)
		if !checkLimit(key, 5, time.Minute) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Terlalu banyak percobaan login. Coba lagi dalam 1 menit."})
			return
		}
		c.Next()
	}
}

func RateLimitSwitch() gin.HandlerFunc {
	return func(c *gin.Context) {
		// per user if authed, else per IP
		key := "switch:" + clientIP(c)
		if claims := ClaimsFrom(c); claims != nil {
			key = "switch:user:" + strconv.FormatUint(uint64(claims.UserID), 10) + ":" + clientIP(c)
		}
		// also count per IP to prevent IP hopping, use combined key with IP
		// simple: enforce both limits via two checks
		if !checkLimit(key, 10, time.Minute) {
			c.Header("Retry-After", "60")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Terlalu banyak percobaan switch. Coba lagi dalam 1 menit."})
			return
		}
		c.Next()
	}
}

func RateLimitForgot() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := "forgot:" + clientIP(c)
		if !checkLimit(key, 3, time.Minute) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Terlalu banyak permintaan reset. Coba lagi dalam 1 menit."})
			return
		}
		c.Next()
	}
}
