package operation_setting

import (
	"errors"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"math"
)

// Rules remain configured while disabled, including for inviters explicitly enabled.
type RebateSetting struct {
	Enabled       bool    `json:"enabled"`
	Rule          string  `json:"rule"` // first or every
	Percent       float64 `json:"percent"`
	MaxPerInvitee int     `json:"max_per_invitee"` // quota; 0 is unlimited
	QuotaPerYuan  float64 `json:"quota_per_yuan"`
	Destination   string  `json:"destination"` // wallet or affiliate
	Online        bool    `json:"online"`
	Subscription  bool    `json:"subscription"`
	Redemption    bool    `json:"redemption"`
	Admin         bool    `json:"admin"`
}

var rebateSetting = RebateSetting{Rule: "first", Destination: "wallet", QuotaPerYuan: 500000, Online: true, Subscription: true}

func init()                            { config.GlobalConfig.Register("rebate_setting", &rebateSetting) }
func GetRebateSetting() *RebateSetting { return &rebateSetting }
func (s RebateSetting) Validate() error {
	if (s.Rule != "first" && s.Rule != "every") || (s.Destination != "wallet" && s.Destination != "affiliate") || !ValidRebatePercent(s.Percent) || math.IsNaN(s.QuotaPerYuan) || math.IsInf(s.QuotaPerYuan, 0) || s.QuotaPerYuan < 0 || s.QuotaPerYuan > float64(common.MaxWalletQuota) || s.MaxPerInvitee < 0 || s.MaxPerInvitee > common.MaxWalletQuota {
		return errors.New("invalid rebate configuration")
	}
	return nil
}
func ValidRebatePercent(p float64) bool {
	return !math.IsNaN(p) && !math.IsInf(p, 0) && p >= 0 && p <= 100
}
