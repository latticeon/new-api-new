package operation_setting

import (
	"errors"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

// Arrival thresholds are site quota units, never inferred from the current balance.
type ArrivalSetting struct {
	Enabled      bool   `json:"enabled"`
	IncludeAdmin bool   `json:"include_admin"`
	Mode         string `json:"mode"` // single or cumulative
	Threshold    int    `json:"threshold"`
	SourceGroup  string `json:"source_group"`
	TargetGroup  string `json:"target_group"`
}

var arrivalSetting = ArrivalSetting{Mode: "cumulative"}

func init()                              { config.GlobalConfig.Register("arrival_setting", &arrivalSetting) }
func GetArrivalSetting() *ArrivalSetting { return &arrivalSetting }

// ValidateArrivalRebateOption validates each independently saved operation option.
// Rules may be edited while disabled and are retained for inviter overrides.
func ValidateArrivalRebateOption(key, value string) error {
	if !strings.HasPrefix(key, "arrival_setting.") && !strings.HasPrefix(key, "rebate_setting.") {
		return nil
	}
	valid := false
	switch key {
	case "arrival_setting.enabled", "arrival_setting.include_admin", "rebate_setting.enabled", "rebate_setting.online", "rebate_setting.subscription", "rebate_setting.redemption", "rebate_setting.admin":
		valid = value == "true" || value == "false"
	case "arrival_setting.mode":
		valid = value == "single" || value == "cumulative"
	case "arrival_setting.source_group", "arrival_setting.target_group":
		valid = len(value) <= 64
	case "arrival_setting.threshold", "rebate_setting.max_per_invitee":
		n, err := strconv.ParseInt(value, 10, 64)
		valid = err == nil && n >= 0 && n <= int64(common.MaxWalletQuota)
	case "rebate_setting.percent":
		n, err := strconv.ParseFloat(value, 64)
		valid = err == nil && ValidRebatePercent(n)
	case "rebate_setting.quota_per_yuan":
		n, err := strconv.ParseFloat(value, 64)
		valid = err == nil && n >= 0 && n <= float64(common.MaxWalletQuota)
	case "rebate_setting.rule":
		valid = value == "first" || value == "every"
	case "rebate_setting.destination":
		valid = value == "wallet" || value == "affiliate"
	}
	if !valid {
		return errors.New("invalid arrival or rebate option")
	}
	return nil
}
