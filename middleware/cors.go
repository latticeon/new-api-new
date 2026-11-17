package middleware

import (
	"net/http"
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

// CORS controls browser access only. It never grants authentication or
// cross-origin cookie credentials; session origin checks remain independent.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			c.Next()
			return
		}
		c.Writer.Header().Add("Vary", "Origin")
		preflight := c.Request.Method == http.MethodOptions && c.GetHeader("Access-Control-Request-Method") != ""
		if preflight {
			c.Writer.Header().Add("Vary", "Access-Control-Request-Method")
			c.Writer.Header().Add("Vary", "Access-Control-Request-Headers")
		}
		rule, found := operation_setting.CORSRuleForPath(c.Request.URL.Path)
		normalizedOrigin, originErr := common.NormalizeOrigin(origin)
		allowed := found && originErr == nil && !strings.ContainsAny(origin, "\\ ,\t?#") && len(c.Request.Header.Values("Origin")) == 1 && (slices.Contains(rule.AllowOrigins, "*") || slices.Contains(rule.AllowOrigins, normalizedOrigin))
		methods := []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
		var requestedHeaders []string
		if preflight {
			allowed = allowed && slices.Contains(methods, c.GetHeader("Access-Control-Request-Method"))
			for header := range strings.SplitSeq(c.GetHeader("Access-Control-Request-Headers"), ",") {
				header = http.CanonicalHeaderKey(strings.TrimSpace(header))
				if header == "" {
					continue
				}
				for _, ch := range header {
					if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || strings.ContainsRune("!#$%&'*+-.^_`|~", ch)) {
						allowed = false
					}
				}
				if !slices.Contains(rule.AllowHeaders, "*") && !slices.Contains(rule.AllowHeaders, header) {
					allowed = false
				}
				requestedHeaders = append(requestedHeaders, header)
			}
		}
		if allowed {
			// Echo the specific origin and headers, including Authorization, rather
			// than relying on wildcard semantics that differ between browsers.
			c.Header("Access-Control-Allow-Origin", origin)
			if preflight {
				c.Header("Access-Control-Allow-Methods", strings.Join(methods, ", "))
				if len(requestedHeaders) > 0 {
					c.Header("Access-Control-Allow-Headers", strings.Join(requestedHeaders, ", "))
				}
				c.Header("Access-Control-Max-Age", "600")
			}
		}
		if preflight {
			if !allowed {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func Version() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-New-Api-Version", common.Version)
		c.Next()
	}
}
