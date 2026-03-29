package middleware

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type bucket struct {
	timestamp int64
	count     int
}

type RateLimiter struct {
	limit int
	mu    sync.Mutex
	m     map[string]*bucket
}

func NewRateLimiter(limit int) *RateLimiter {
	if limit <= 0 {
		limit = 50
	}
	return &RateLimiter{limit: limit, m: make(map[string]*bucket, 1024)}
}

func (r *RateLimiter) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		now := time.Now().Unix()
		ip := c.ClientIP()

		r.mu.Lock()
		b, ok := r.m[ip]
		if !ok {
			r.m[ip] = &bucket{timestamp: now, count: 1}
			r.mu.Unlock()
			c.Next()
			return
		}

		if b.timestamp != now {
			b.timestamp = now
			b.count = 1
			r.mu.Unlock()
			c.Next()
			return
		}

		b.count++
		allow := b.count <= r.limit
		r.mu.Unlock()

		if !allow {
			c.JSON(429, gin.H{"message": "请求过于频繁，请稍后重试"})
			c.Abort()
			return
		}
		c.Next()
	}
}
