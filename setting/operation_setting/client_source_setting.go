package operation_setting

import (
	"strings"
	"sync"
	"unicode"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

const ClientSourceAliasesOption = "client_source_setting.aliases"

// ClientSourceAlias is display-only; User-Agent is supplied by the caller and
// must never be used as an authentication or trust signal.
type ClientSourceAlias struct {
	Pattern string `json:"pattern"`
	Alias   string `json:"alias"`
}

var clientSourceSetting = struct {
	Aliases string `json:"aliases"`
}{Aliases: "[]"}

var clientSourceCache struct {
	sync.Mutex
	raw   string
	rules []ClientSourceAlias
}

func init() {
	config.GlobalConfig.Register("client_source_setting", &clientSourceSetting)
}

// ParseClientSourceAliases bounds both parsing and substring matching. Patterns
// are literal, case-insensitive substrings, evaluated in the configured order.
func ParseClientSourceAliases(raw string) ([]ClientSourceAlias, error) {
	var rules []ClientSourceAlias
	if len(raw) > 65536 || common.UnmarshalJsonStr(raw, &rules) != nil || rules == nil || len(rules) > 100 {
		return nil, common.NewMessage("Invalid User-Agent aliases")
	}
	for i := range rules {
		rule := &rules[i]
		rule.Pattern = strings.TrimSpace(rule.Pattern)
		rule.Alias = strings.TrimSpace(rule.Alias)
		if rule.Pattern == "" || len(rule.Pattern) > 256 || rule.Alias == "" || len(rule.Alias) > 128 ||
			strings.ContainsFunc(rule.Pattern, unicode.IsControl) || strings.ContainsFunc(rule.Alias, unicode.IsControl) {
			return nil, common.NewMessage("Invalid User-Agent aliases")
		}
		rule.Pattern = strings.ToLower(rule.Pattern)
	}
	return rules, nil
}

// ClientSourceLabel reads the synchronized option snapshot instead of the
// reflection-backed config, which can be updated concurrently. Invalid saved
// rules fall back to the raw User-Agent; historical requests remain unchanged.
func ClientSourceLabel(userAgent string) string {
	if userAgent == "" {
		return ""
	}
	common.OptionMapRWMutex.RLock()
	raw, ok := common.OptionMap[ClientSourceAliasesOption]
	common.OptionMapRWMutex.RUnlock()
	if !ok {
		raw = "[]"
	}
	clientSourceCache.Lock()
	defer clientSourceCache.Unlock()
	if raw != clientSourceCache.raw {
		clientSourceCache.raw = raw
		clientSourceCache.rules, _ = ParseClientSourceAliases(raw)
	}
	lower := strings.ToLower(userAgent)
	for _, rule := range clientSourceCache.rules {
		if strings.Contains(lower, rule.Pattern) {
			return rule.Alias
		}
	}
	return userAgent
}
