package model

import (
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
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

func TestLogOtherScopesAndMerges(t *testing.T) {
	var other LogOther

	assert.True(t, other.SetPublic("request_path", "/v1/chat/completions"))
	other.MergePublic(map[string]any{
		"zero": 0,
	})
	assert.True(t, other.SetAdmin("use_channel", []string{"channel-a"}))
	other.MergeAdmin(map[string]any{
		"rejected": false,
	})
	assert.True(t, other.SetRoot("upstream_request_id", "upstream-private"))
	other.MergeRoot(map[string]any{
		"generation": 0,
	})
	assert.True(t, other.SetAudit("method", "POST"))
	other.MergeAudit(map[string]any{
		"success": false,
	})

	require.JSONEq(t, `{
		"request_path": "/v1/chat/completions",
		"zero": 0,
		"admin_info": {
			"use_channel": ["channel-a"],
			"rejected": false
		},
		"root_info": {
			"upstream_request_id": "upstream-private",
			"generation": 0
		},
		"audit_info": {
			"method": "POST",
			"success": false
		}
	}`, other.JSONString())
}

func TestLogOtherRejectsSensitivePublicFields(t *testing.T) {
	other := NewLogOther()

	for _, key := range []string{
		"admin_info",
		"root_info",
		"audit_info",
		"channel_id",
		"channel_name",
		"channel_type",
		"reject_reason",
	} {
		assert.False(t, other.SetPublic(key, "must-not-leak"), key)
	}
	other.MergePublic(map[string]any{
		"request_path": "/v1/responses",
		"channel_name": "still-must-not-leak",
		"admin_info":   map[string]any{"secret": true},
	})

	require.JSONEq(t, `{"request_path":"/v1/responses"}`, other.JSONString())
	require.JSONEq(t, `{}`, NewLogOther().JSONString())
}

func TestLogOtherJSONStringDoesNotMutateReceiver(t *testing.T) {
	other := NewLogOther()
	require.True(t, other.SetPublic("request_path", "/v1/chat/completions"))
	require.True(t, other.SetAdmin("rejected", false))

	before := other.Snapshot()
	first := other.JSONString()
	after := other.Snapshot()
	second := other.JSONString()

	require.Equal(t, before, after)
	require.Equal(t, first, second)
}

func TestClientSourceRecordsOnlyRawUserAgent(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	c.Request.Header.Set("User-Agent", "CherryStudio/1.0 (Desktop)")
	c.Request.Header.Set("X-Client-Source", "spoofed-source")
	other := NewLogOther()
	other.recordClientSource(c)
	assert.Equal(t, "CherryStudio/1.0 (Desktop)", other.Snapshot()["user_agent"])
	assert.NotContains(t, other.Snapshot(), "client_source")
	assert.NotContains(t, other.JSONString(), "spoofed-source")

	c.Request.Header.Del("User-Agent")
	unknown := NewLogOther()
	unknown.recordClientSource(c)
	unknown.recordClientSource(nil)
	assert.Empty(t, unknown.Snapshot())
}

func TestClientSourceAliasesProjectDynamicallyWithoutLosingMetadata(t *testing.T) {
	common.OptionMapRWMutex.Lock()
	original := common.OptionMap
	common.OptionMap = map[string]string{
		operation_setting.ClientSourceAliasesOption: `[{"pattern":"CHERRY","alias":"Cherry Studio"},{"pattern":"studio","alias":"Other"}]`,
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = original
		common.OptionMapRWMutex.Unlock()
	})
	const stored = `{"user_agent":"CherryStudio/1.0","public_id":9007199254740993,"admin_info":{"admin_id":9007199254740995},"root_info":{"generation":18446744073709551615}}`
	for _, visibility := range []logOtherVisibility{logOtherVisibilityUser, logOtherVisibilityAdmin, logOtherVisibilityRoot} {
		projected := formatLogOtherJSON(stored, visibility)
		assert.Contains(t, projected, `"user_agent":"CherryStudio/1.0"`)
		assert.Contains(t, projected, `"client_source":"Cherry Studio"`)
		assert.Contains(t, projected, `"public_id":9007199254740993`)
		if visibility == logOtherVisibilityUser {
			assert.NotContains(t, projected, "admin_info")
		} else {
			assert.Contains(t, projected, `"admin_id":9007199254740995`)
		}
		if visibility == logOtherVisibilityRoot {
			assert.Contains(t, projected, `"generation":18446744073709551615`)
		} else {
			assert.NotContains(t, projected, "root_info")
		}
	}
	common.OptionMapRWMutex.Lock()
	common.OptionMap[operation_setting.ClientSourceAliasesOption] = `[{"pattern":"cherry","alias":"Renamed"}]`
	common.OptionMapRWMutex.Unlock()
	assert.Contains(t, formatLogOtherJSON(stored, logOtherVisibilityRoot), `"client_source":"Renamed"`)
	common.OptionMapRWMutex.Lock()
	common.OptionMap[operation_setting.ClientSourceAliasesOption] = `invalid`
	common.OptionMapRWMutex.Unlock()
	assert.Contains(t, formatLogOtherJSON(stored, logOtherVisibilityRoot), `"client_source":"CherryStudio/1.0"`)
	const untouched = `{ "public_id": 9007199254740993 }`
	assert.Equal(t, untouched, formatLogOtherJSON(untouched, logOtherVisibilityRoot))
	const alreadyProjected = `{ "user_agent":"curl/8.0", "client_source":"curl/8.0" }`
	assert.Equal(t, alreadyProjected, formatLogOtherJSON(alreadyProjected, logOtherVisibilityRoot))
}

func TestClientSourceAliasValidation(t *testing.T) {
	for _, raw := range []string{
		`null`, `{}`, `invalid`, `[{"pattern":" ","alias":"Client"}]`,
		`[{"pattern":"client","alias":""}]`, `[{"pattern":"cli\nent","alias":"Client"}]`,
		`[{"pattern":"client","alias":"Cli\u0000ent"}]`,
		`[{"pattern":"` + strings.Repeat("a", 257) + `","alias":"Client"}]`,
		`[{"pattern":"client","alias":"` + strings.Repeat("a", 129) + `"}]`,
		strings.Repeat(" ", 65537),
		`[` + strings.Repeat(`{"pattern":"client","alias":"Client"},`, 100) + `{"pattern":"client","alias":"Client"}]`,
	} {
		_, err := operation_setting.ParseClientSourceAliases(raw)
		require.Error(t, err)
	}
	rules, err := operation_setting.ParseClientSourceAliases(`[{"pattern":" CURL/ ","alias":" curl "},{"pattern":"[.*]","alias":"Literal"}]`)
	require.NoError(t, err)
	assert.Equal(t, "curl/", rules[0].Pattern)
	assert.Equal(t, "curl", rules[0].Alias)
	assert.Equal(t, "[.*]", rules[1].Pattern)
	rules, err = operation_setting.ParseClientSourceAliases(`[]`)
	require.NoError(t, err)
	assert.Empty(t, rules)
}

func TestClientSourceDatabaseMatrix(t *testing.T) {
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
			db, err := gorm.Open(driver, &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: "client_source_test_"}})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			previousDB, previousLogDB := DB, LOG_DB
			previousRedis, previousConsume, previousExport := common.RedisEnabled, common.LogConsumeEnabled, common.DataExportEnabled
			common.OptionMapRWMutex.Lock()
			previousMap := common.OptionMap
			common.OptionMap = make(map[string]string)
			common.OptionMapRWMutex.Unlock()
			DB, LOG_DB = db, db
			common.RedisEnabled, common.LogConsumeEnabled, common.DataExportEnabled = false, true, false
			t.Cleanup(func() {
				DB, LOG_DB = previousDB, previousLogDB
				common.RedisEnabled, common.LogConsumeEnabled, common.DataExportEnabled = previousRedis, previousConsume, previousExport
				common.OptionMapRWMutex.Lock()
				common.OptionMap = previousMap
				common.OptionMapRWMutex.Unlock()
				assert.NoError(t, db.Migrator().DropTable(&Log{}, &Option{}, &SystemTask{}))
				assert.NoError(t, db.Migrator().DropTable("client_source_test_users"))
				assert.NoError(t, sqlDB.Close())
			})
			require.NoError(t, db.AutoMigrate(&Log{}, &Option{}, &SystemTask{}))
			// The recording path reads only the user's IP logging preference.
			require.NoError(t, db.Table("client_source_test_users").AutoMigrate(&struct {
				Id        int
				Setting   string
				DeletedAt gorm.DeletedAt
			}{}))
			require.NoError(t, db.Table("client_source_test_users").Create(map[string]any{"id": 1, "setting": "{}"}).Error)
			var version string
			query := "SELECT version()"
			if dialect == "sqlite" {
				query = "SELECT sqlite_version()"
			}
			require.NoError(t, db.Raw(query).Scan(&version).Error)
			t.Logf("database version: %s", version)
			const rules = `[{"pattern":"CHERRY","alias":"Cherry Studio"},{"pattern":"studio","alias":"Other"}]`
			require.NoError(t, UpdateOption(operation_setting.ClientSourceAliasesOption, rules))
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
			c.Request.Header.Set("User-Agent", "CherryStudio/1.0 (Desktop)")
			c.Request.Header.Set("X-Client-Source", "spoofed-source")
			c.Set("username", "source-test")
			RecordConsumeLog(c, 1, RecordConsumeLogParams{ModelName: "source-test"})
			RecordErrorLog(c, 1, 0, "source-test", "", "upstream failed", 0, 0, false, "", nil)
			c.Request.Header.Del("User-Agent")
			RecordConsumeLog(c, 1, RecordConsumeLogParams{ModelName: "unknown-client"})
			var logs []*Log
			require.NoError(t, db.Order("id").Find(&logs).Error)
			require.Len(t, logs, 3)
			assert.Equal(t, LogTypeConsume, logs[0].Type)
			assert.Equal(t, LogTypeError, logs[1].Type)
			for _, log := range logs[:2] {
				assert.Contains(t, log.Other, `"user_agent":"CherryStudio/1.0 (Desktop)"`)
				assert.NotContains(t, log.Other, "client_source")
				assert.NotContains(t, log.Other, "spoofed-source")
			}
			assert.NotContains(t, logs[2].Other, "user_agent")
			FormatRootLogs(logs)
			assert.Contains(t, logs[0].Other, `"client_source":"Cherry Studio"`)
			assert.Contains(t, logs[1].Other, `"client_source":"Cherry Studio"`)
			assert.NotContains(t, logs[2].Other, "client_source")
			require.Error(t, UpdateOption(operation_setting.ClientSourceAliasesOption, `[{"pattern":"","alias":"Invalid"}]`))
			var saved Option
			require.NoError(t, db.Where(&Option{Key: operation_setting.ClientSourceAliasesOption}).First(&saved).Error)
			assert.Equal(t, rules, saved.Value)
			require.NoError(t, UpdateOption(operation_setting.ClientSourceAliasesOption, `[{"pattern":"cherry","alias":"Renamed"}]`))
			FormatRootLogs(logs)
			assert.Contains(t, logs[0].Other, `"client_source":"Renamed"`)
			require.NoError(t, UpdateOption(operation_setting.ClientSourceAliasesOption, `[]`))
			FormatRootLogs(logs)
			assert.Contains(t, logs[0].Other, `"client_source":"CherryStudio/1.0 (Desktop)"`)
			var stored Log
			require.NoError(t, db.First(&stored, logs[0].Id).Error)
			assert.NotContains(t, stored.Other, "client_source", "display changes must not rewrite history")
		})
	}
}
