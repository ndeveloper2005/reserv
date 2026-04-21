package appresult

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type UnsignedResponse struct {
	Message interface{} `json:"message"`
}

type appHandler func(w http.ResponseWriter, r *http.Request) error

func HeaderContentTypeJson() (string, string) {
	return "Content-Type", "application/json"
}
func AccessControlAllow() (string, string) {
	return "Access-Control-Allow-Origin", "*"
}

func respondWithError(c *gin.Context, code int, message interface{}) {
	c.AbortWithStatusJSON(code, gin.H{"error": message})
}

func BaseURLMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		baseURL := fmt.Sprintf("%s://%s", scheme, c.Request.Host)
		c.Set("baseURL", baseURL)

		c.Next()
	}
}
