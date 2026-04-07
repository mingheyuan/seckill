package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			id = randomID()
		}
		c.Set("request_id", id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}

func IPAccessControl(rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if blackExist,_:=rdb.SIsMember(c.Request.Context(),"ip:blacklist",ip).Result();blackExist {
			c.JSON(http.StatusForbidden, gin.H{"message": "ip is blocked"})
			c.Abort()
			return
		}
		if whiteLen,_:=rdb.SCard(c.Request.Context(),"ip:whitelist").Result();whiteLen==0{
			c.Next()
			return
		}
		if  whiteExist, _ := rdb.SIsMember(c.Request.Context(), "ip:whitelist", ip).Result();!whiteExist{
				c.JSON(http.StatusForbidden, gin.H{"message": "ip is not in whitelist"})
				c.Abort()
				return
		}
		c.Next()
	}
}

func toSet(items []string) map[string]struct{} {
	if len(items) == 0 {
		return nil
	}
	m := make(map[string]struct{}, len(items))
	for i := range items {
		if items[i] != "" {
			m[items[i]] = struct{}{}
		}
	}
	return m
}

func randomID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "req-fallback"
	}
	return hex.EncodeToString(b)
}
