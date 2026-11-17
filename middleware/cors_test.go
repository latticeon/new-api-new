package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestConfigurableCORS(t *testing.T) {
	common.OptionMapRWMutex.Lock()
	previous, existed := common.OptionMap[operation_setting.CORSRulesOption]
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	common.OptionMap[operation_setting.CORSRulesOption] = `[{"path_prefix":"/","allow_origins":["*"],"allow_headers":["*"]},{"path_prefix":"/api/","allow_origins":[],"allow_headers":[]},{"path_prefix":"/api/user/","allow_origins":["https://client.example"],"allow_headers":["Authorization","X-Client-Session"]}]`
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		defer common.OptionMapRWMutex.Unlock()
		if existed {
			common.OptionMap[operation_setting.CORSRulesOption] = previous
		} else {
			delete(common.OptionMap, operation_setting.CORSRulesOption)
		}
	})
	r := gin.New()
	r.Use(CORS())
	calls := 0
	r.Any("/*path", func(c *gin.Context) { calls++; c.Status(http.StatusOK) })
	for _, tc := range []struct {
		name, path, origin, method, headers string
		status                              int
		allowed                             bool
	}{
		{"relay wildcard custom header", "/v1/chat/completions", "https://other.example", "POST", "Authorization, X-Custom", 204, true},
		{"specific route", "/api/user/self", "https://client.example", "GET", "authorization, x-client-session", 204, true},
		{"normalized origin", "/api/user/self", "https://CLIENT.example:443", "GET", "Authorization", 204, true},
		{"malformed origin", "/v1/chat/completions", "https://client.example?", "POST", "", 403, false},
		{"malformed requested header", "/v1/chat/completions", "https://client.example", "POST", "X Invalid", 403, false},
		{"denied origin", "/api/user/self", "https://client.example.evil", "GET", "Authorization", 403, false},
		{"denied header", "/api/user/self", "https://client.example", "GET", "Authorization, X-Other", 403, false},
		{"closed admin route", "/api/option/", "https://client.example", "PUT", "Authorization", 403, false},
		{"unsupported method", "/v1/chat/completions", "https://client.example", "TRACE", "", 403, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodOptions, tc.path, nil)
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("Access-Control-Request-Method", tc.method)
			req.Header.Set("Access-Control-Request-Headers", tc.headers)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, tc.status, w.Code)
			if tc.allowed {
				assert.Equal(t, tc.origin, w.Header().Get("Access-Control-Allow-Origin"))
			} else {
				assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
			}
			assert.Empty(t, w.Header().Get("Access-Control-Allow-Credentials"))
			assert.Contains(t, w.Header().Values("Vary"), "Access-Control-Request-Headers")
		})
	}
	assert.Zero(t, calls, "preflight must not execute business handlers")
	req := httptest.NewRequest(http.MethodGet, "/api/user/self", nil)
	req.Header.Set("Origin", "https://client.example")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, "https://client.example", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, 1, calls)
	common.OptionMapRWMutex.Lock()
	common.OptionMap[operation_setting.CORSRulesOption] = `[]`
	common.OptionMapRWMutex.Unlock()
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"), "updated policy applies without rebuilding router")
	assert.Equal(t, 2, calls, "CORS does not replace server authorization")
	common.OptionMapRWMutex.Lock()
	common.OptionMap[operation_setting.CORSRulesOption] = `invalid`
	common.OptionMapRWMutex.Unlock()
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"), "invalid stored policy fails closed")
	noOrigin := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, noOrigin)
	assert.Equal(t, http.StatusOK, w.Code, "non-browser clients remain unaffected")
	assert.Equal(t, 4, calls)
	common.OptionMapRWMutex.Lock()
	delete(common.OptionMap, operation_setting.CORSRulesOption)
	common.OptionMapRWMutex.Unlock()
	policy, ok := operation_setting.CORSRuleForPath("/api/option/")
	require.True(t, ok)
	assert.Empty(t, policy.AllowOrigins, "management API is closed by default")
	policy, ok = operation_setting.CORSRuleForPath("/api/usage/token")
	require.True(t, ok)
	assert.Equal(t, []string{"*"}, policy.AllowOrigins)
}

func TestCORSRuleValidation(t *testing.T) {
	_, err := operation_setting.ParseCORSRules(operation_setting.DefaultCORSRules)
	require.NoError(t, err)
	for _, raw := range []string{
		`null`, `{}`, `[{"path_prefix":"api/"}]`,
		`[{"path_prefix":"/"},{"path_prefix":"/"}]`,
		`[{"path_prefix":"/","allow_origins":["null"]}]`,
		`[{"path_prefix":"/","allow_origins":["https://example.com/private"]}]`,
		`[{"path_prefix":"/","allow_origins":["https://user:password@example.com"]}]`,
		`[{"path_prefix":"/","allow_headers":["X-Test\r\nInjected"]}]`,
	} {
		_, err := operation_setting.ParseCORSRules(raw)
		assert.Error(t, err, raw)
	}
}

func TestCORSOptionDatabaseMatrix(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var driver gorm.Dialector
			switch dialect {
			case "sqlite":
				driver = sqlite.Open(":memory:")
			case "mysql":
				dsn := os.Getenv("TEST_MYSQL_DSN")
				if dsn == "" {
					t.Skip("TEST_MYSQL_DSN not configured")
				}
				driver = mysql.Open(dsn)
			case "postgres":
				dsn := os.Getenv("TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("TEST_POSTGRES_DSN not configured")
				}
				driver = postgres.Open(dsn)
			}
			db, err := gorm.Open(driver, &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: "cors_option_test_"}})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			previousDB := model.DB
			common.OptionMapRWMutex.Lock()
			previousMap := common.OptionMap
			common.OptionMap = make(map[string]string)
			common.OptionMapRWMutex.Unlock()
			model.DB = db
			t.Cleanup(func() {
				model.DB = previousDB
				common.OptionMapRWMutex.Lock()
				common.OptionMap = previousMap
				common.OptionMapRWMutex.Unlock()
				require.NoError(t, db.Migrator().DropTable(&model.Option{}, &model.SystemTask{}))
				require.NoError(t, sqlDB.Close())
			})
			require.NoError(t, db.AutoMigrate(&model.Option{}, &model.SystemTask{}))
			var version string
			query := "SELECT version()"
			if dialect == "sqlite" {
				query = "SELECT sqlite_version()"
			}
			require.NoError(t, db.Raw(query).Scan(&version).Error)
			t.Logf("database version: %s", version)
			const raw = `[{"path_prefix":"/v1/","allow_origins":["https://client.example:8443"],"allow_headers":["Authorization","X-Client-Session"]}]`
			require.NoError(t, model.UpdateOption(operation_setting.CORSRulesOption, raw))
			var saved model.Option
			require.NoError(t, db.Where(&model.Option{Key: operation_setting.CORSRulesOption}).First(&saved).Error)
			assert.Equal(t, raw, saved.Value)
			policy, ok := operation_setting.CORSRuleForPath("/v1/chat/completions")
			require.True(t, ok)
			assert.Equal(t, []string{"https://client.example:8443"}, policy.AllowOrigins)
			assert.Equal(t, []string{"Authorization", "X-Client-Session"}, policy.AllowHeaders)
			require.Error(t, model.UpdateOption(operation_setting.CORSRulesOption, `[{"path_prefix":"/","allow_origins":["https://evil.example/path"]}]`))
			var after model.Option
			require.NoError(t, db.Where(&model.Option{Key: operation_setting.CORSRulesOption}).First(&after).Error)
			assert.Equal(t, raw, after.Value)
			require.NoError(t, model.UpdateOption(operation_setting.CORSRulesOption, `[]`))
			_, ok = operation_setting.CORSRuleForPath("/v1/chat/completions")
			assert.False(t, ok)
		})
	}
}
