package operation_setting

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

const CORSRulesOption = "cors_setting.rules"
const DefaultCORSRules = `[{"path_prefix":"/","allow_origins":["*"],"allow_headers":["*"]},{"path_prefix":"/api/","allow_origins":[],"allow_headers":[]},{"path_prefix":"/api/usage/","allow_origins":["*"],"allow_headers":["*"]},{"path_prefix":"/api/log/token","allow_origins":["*"],"allow_headers":["*"]}]`

type CORSRule struct {
	PathPrefix   string   `json:"path_prefix"`
	AllowOrigins []string `json:"allow_origins"`
	AllowHeaders []string `json:"allow_headers"`
}

var corsSetting = struct {
	Rules string `json:"rules"`
}{Rules: DefaultCORSRules}
var corsRulesCache struct {
	sync.Mutex
	raw   string
	rules []CORSRule
}

func init() { config.GlobalConfig.Register("cors_setting", &corsSetting) }

// ParseCORSRules validates the complete policy before it can be persisted.
func ParseCORSRules(raw string) ([]CORSRule, error) {
	var rules []CORSRule
	if len(raw) > 65536 || common.UnmarshalJsonStr(raw, &rules) != nil || rules == nil || len(rules) > 100 {
		return nil, fmt.Errorf("invalid CORS rules")
	}
	seen := make(map[string]bool)
	for i := range rules {
		r := &rules[i]
		if !strings.HasPrefix(r.PathPrefix, "/") || strings.ContainsAny(r.PathPrefix, "?#\r\n") || seen[r.PathPrefix] || len(r.AllowOrigins) > 100 || len(r.AllowHeaders) > 100 {
			return nil, fmt.Errorf("invalid CORS path prefix or list")
		}
		seen[r.PathPrefix] = true
		for j, origin := range r.AllowOrigins {
			if origin == "*" {
				continue
			}
			normalized, err := common.NormalizeOrigin(origin)
			if err != nil || strings.ContainsAny(origin, "\\ ,\t?#") {
				return nil, fmt.Errorf("invalid CORS origin")
			}
			r.AllowOrigins[j] = normalized
		}
		for j, header := range r.AllowHeaders {
			if header == "*" {
				continue
			}
			if header == "" {
				return nil, fmt.Errorf("invalid CORS header")
			}
			for _, ch := range header {
				if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || strings.ContainsRune("!#$%&'*+-.^_`|~", ch)) {
					return nil, fmt.Errorf("invalid CORS header")
				}
			}
			r.AllowHeaders[j] = http.CanonicalHeaderKey(header)
		}
	}
	return rules, nil
}

// CORSRuleForPath uses the option-map lock, not the mutable reflection config.
// Invalid stored policies fail closed; a missing option uses the shipped policy.
func CORSRuleForPath(path string) (CORSRule, bool) {
	common.OptionMapRWMutex.RLock()
	raw, ok := common.OptionMap[CORSRulesOption]
	common.OptionMapRWMutex.RUnlock()
	if !ok {
		raw = DefaultCORSRules
	}
	corsRulesCache.Lock()
	defer corsRulesCache.Unlock()
	if raw != corsRulesCache.raw || corsRulesCache.rules == nil {
		corsRulesCache.raw = raw
		corsRulesCache.rules, _ = ParseCORSRules(raw)
	}
	var match CORSRule
	found := false
	for _, rule := range corsRulesCache.rules {
		if strings.HasPrefix(path, rule.PathPrefix) && (!found || len(rule.PathPrefix) > len(match.PathPrefix)) {
			match, found = rule, true
		}
	}
	return match, found
}
