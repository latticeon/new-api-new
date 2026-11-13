package operation_setting

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

const LogMaxRangeDaysOptionKey = "log_setting.max_range_days"

// LogSetting limits how far regular users can reach into usage logs.
type LogSetting struct {
	// UserExportEnabled lets regular users export their own usage logs.
	// Administrators can always export.
	UserExportEnabled bool `json:"user_export_enabled"`
	// MaxRangeDays is the longest time range a user can search in their logs
	// and dashboard, and the longest range any export can cover.
	MaxRangeDays         int  `json:"max_range_days"`
	AutoCleanupEnabled   bool `json:"auto_cleanup_enabled"`
	CleanupRetentionDays int  `json:"cleanup_retention_days"`
	// CleanupScope is a closed allowlist: consume_error, consume, or error.
	CleanupScope string `json:"cleanup_scope"`
}

var logSetting = LogSetting{
	UserExportEnabled:    true,
	MaxRangeDays:         365,
	CleanupRetentionDays: 30,
	CleanupScope:         "consume_error",
}

func init() {
	config.GlobalConfig.Register("log_setting", &logSetting)
}

func GetLogSetting() *LogSetting {
	return &logSetting
}

func (s *LogSetting) MaxRangeSeconds() int64 {
	return int64(max(s.MaxRangeDays, 1)) * 24 * 60 * 60
}

func ValidateLogMaxRangeDays(value string) error {
	days, err := strconv.Atoi(value)
	if err != nil || days < 1 {
		return common.NewMessage("The longest log time range must be at least 1 day")
	}
	return nil
}

// ValidateLogCleanupOption rejects invalid saved values as well as unsafe ranges.
func ValidateLogCleanupOption(key, value string) error {
	switch key {
	case "log_setting.auto_cleanup_enabled":
		if value != "true" && value != "false" {
			return common.NewMessage("Invalid log cleanup setting")
		}
	case "log_setting.cleanup_retention_days":
		days, err := strconv.Atoi(value)
		if err != nil || days < 1 || days > 36500 {
			return common.NewMessage("Log retention must be between 1 and 36500 days")
		}
	case "log_setting.cleanup_scope":
		if value != "consume_error" && value != "consume" && value != "error" {
			return common.NewMessage("Only consumption and error logs can be cleaned")
		}
	}
	return nil
}

func (s *LogSetting) CleanupValid() bool {
	return s.CleanupRetentionDays >= 1 && s.CleanupRetentionDays <= 36500 &&
		(s.CleanupScope == "consume_error" || s.CleanupScope == "consume" || s.CleanupScope == "error")
}
