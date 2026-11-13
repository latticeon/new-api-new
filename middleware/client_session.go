package middleware

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// ClientSessionHeader carries the short-lived access token, never a refresh
// secret. Authorization: Bearer is preferred; this header is an alternative.
const ClientSessionHeader = "X-Client-Session"

var errClientSessionRouteDenied = errors.New("client session route denied")

// Deliberately method + registered route, not URL-prefix access to management
// APIs. New dashboard endpoints remain denied even for administrator clients.
var clientSessionRoutes = map[string]bool{
	"PUT /api/user/self":                    true,
	"GET /api/user/self":                    true,
	"GET /api/user/models":                  true,
	"GET /api/user/aff":                     true,
	"POST /api/user/aff_transfer":           true,
	"GET /api/user/checkin":                 true,
	"POST /api/user/checkin":                true,
	"GET /api/log/self":                     true,
	"GET /api/log/self/stat":                true,
	"GET /api/user/sessions":                true,
	"DELETE /api/user/sessions/:sid":        true,
	"GET /api/user/topup/info":              true,
	"GET /api/user/topup/self":              true,
	"POST /api/user/topup":                  true,
	"POST /api/user/amount":                 true,
	"POST /api/user/pay":                    true,
	"POST /api/user/stripe/amount":          true,
	"POST /api/user/stripe/pay":             true,
	"POST /api/user/creem/pay":              true,
	"POST /api/user/waffo/pay":              true,
	"GET /api/subscription/plans":           true,
	"GET /api/subscription/self":            true,
	"PUT /api/subscription/self/preference": true,
	"POST /api/subscription/epay/pay":       true,
	"POST /api/subscription/stripe/pay":     true,
	"POST /api/subscription/creem/pay":      true,
}

func IsClientSessionAllowedRoute(method, fullPath string) bool {
	return fullPath != "" && (fullPath == "/v1" || strings.HasPrefix(fullPath, "/v1/") || clientSessionRoutes[method+" "+fullPath])
}

func validateClientSessionRequest(c *gin.Context, session *model.UserSession, user *model.UserBase) error {
	if !strings.HasPrefix(session.LoginMethod, service.ClientLoginMethodPrefix) {
		return service.ErrAuthTokenInvalid
	}
	if !IsClientSessionAllowedRoute(c.Request.Method, c.FullPath()) {
		return errClientSessionRouteDenied
	}
	if supplied := c.GetHeader("New-Api-User"); supplied != "" {
		id, err := strconv.Atoi(supplied)
		if err != nil || id != user.Id {
			return service.ErrAuthTokenInvalid
		}
	}
	c.Set("client_session", true)
	return nil
}

// clientRelayAuth bridges a verified client session into the same temporary
// token context used by Playground. The user's group drives Distribute; user
// wallet/subscription billing is unchanged, and no persisted API token is minted.
func clientRelayAuth(c *gin.Context, raw string) {
	identity, err := service.ParseAccessToken(raw)
	if err != nil {
		writeDashboardAuthError(c, err)
		return
	}
	session, user, err := service.ValidateLoginSession(identity)
	if err == nil {
		err = validateClientSessionRequest(c, session, user)
	}
	if err != nil {
		writeDashboardAuthError(c, err)
		return
	}
	if !validUserInfo(user.Username, user.Role) {
		writeDashboardAuthError(c, service.ErrAuthTokenInvalid)
		return
	}
	setDashboardAuthContext(c, user, identity, false)
	common.SetContextKey(c, constant.ContextKeyUsingGroup, user.Group)
	if err := SetupContextForToken(c, &model.Token{UserId: user.Id, Name: "client-" + user.Group, Group: user.Group}); err != nil {
		return
	}
	// Server-owned context only: GenRelayInfo reuses the user-only billing
	// path, without rewriting the request URL or trusting a request flag.
	c.Set("client_session_relay", true)
	// Never expose login credentials to upstream passthrough or request plugins.
	c.Request.Header.Del("Authorization")
	c.Request.Header.Del(ClientSessionHeader)
	c.Request.Header.Del("Cookie")
	c.Next()
}

func rejectMixedClientCredentials(c *gin.Context) {
	abortWithWebMessage(c, http.StatusUnauthorized, "AUTH_UNAUTHORIZED", common.NewMessage("Unauthorized, invalid access token"))
}
