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
// Arrival rules may be edited while disabled.
func ValidateArrivalRebateOption(key, value string) error {
	if !strings.HasPrefix(key, "arrival_setting.") {
		return nil
	}
	valid := false
	switch key {
	case "arrival_setting.enabled", "arrival_setting.include_admin":
		valid = value == "true" || value == "false"
	case "arrival_setting.mode":
		valid = value == "single" || value == "cumulative"
	case "arrival_setting.source_group", "arrival_setting.target_group":
		valid = len(value) <= 64
	case "arrival_setting.threshold":
		n, err := strconv.ParseInt(value, 10, 64)
		valid = err == nil && n >= 0 && n <= int64(common.MaxWalletQuota)
	}
	if !valid {
		return errors.New("invalid arrival or rebate option")
	}
	return nil
}
