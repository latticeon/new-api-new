package common

import (
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/relaykit/types"
)

const (
	OptionErrorMessageRewriteEnabled = "ErrorMessageRewriteEnabled"
	OptionErrorMessageRewriteRules   = "ErrorMessageRewriteRules"
)

type ErrorMessageSource = types.ErrorMessageSource

const (
	ErrorMessageSourceAll      = types.ErrorMessageSourceAll
	ErrorMessageSourceUpstream = types.ErrorMessageSourceUpstream
	ErrorMessageSourceLocal    = types.ErrorMessageSourceLocal
)

type ErrorMessageRewriteRule struct {
	Key       string             `json:"key"`
	Enabled   bool               `json:"enabled"`
	Priority  int                `json:"priority"`
	Source    ErrorMessageSource `json:"source"`
	MatchType string             `json:"match_type"`
	Patterns  []string           `json:"patterns"`
	Replace   string             `json:"replace"`
}

type compiledErrorMessageRewriteRule struct {
	ErrorMessageRewriteRule
	regexps []*regexp.Regexp
}

var requestIDSuffixRegexp = regexp.MustCompile(`\s+\(request id: [^)]+\)$`)
var errorMessageRewriteCache struct {
	sync.Mutex
	raw   string
	rules []compiledErrorMessageRewriteRule
}

// Compile once per configuration value, not once per error or regex match.
func compileErrorMessageRewriteRules(raw string) ([]compiledErrorMessageRewriteRule, error) {
	if strings.TrimSpace(raw) == "" {
		raw = "[]"
	}
	if !strings.HasPrefix(strings.TrimSpace(raw), "[") {
		return nil, fmt.Errorf("Error rewrite rules must be a JSON array")
	}
	var rules []ErrorMessageRewriteRule
	if err := UnmarshalJsonStr(raw, &rules); err != nil {
		return nil, fmt.Errorf("Invalid error rewrite rules JSON: %w", err)
	}
	compiled := make([]compiledErrorMessageRewriteRule, 0, len(rules))
	for i, rule := range rules {
		rule.Key = strings.TrimSpace(rule.Key)
		rule.Replace = strings.TrimSpace(rule.Replace)
		if rule.Replace == "" {
			return nil, fmt.Errorf("Error rewrite rule %d: replacement cannot be empty", i+1)
		}
		if rule.Source == "" {
			rule.Source = ErrorMessageSourceAll
		}
		if !slices.Contains([]ErrorMessageSource{ErrorMessageSourceAll, ErrorMessageSourceUpstream, ErrorMessageSourceLocal}, rule.Source) {
			return nil, fmt.Errorf("Error rewrite rule %d: invalid source", i+1)
		}
		if rule.MatchType == "" {
			rule.MatchType = "contains_any"
		}
		if !slices.Contains([]string{"exact_any", "contains_any", "regex_any"}, rule.MatchType) {
			return nil, fmt.Errorf("Error rewrite rule %d: invalid match type", i+1)
		}
		next := compiledErrorMessageRewriteRule{ErrorMessageRewriteRule: rule}
		next.Patterns = nil
		for _, pattern := range rule.Patterns {
			pattern = strings.TrimSpace(pattern)
			if pattern == "" {
				continue
			}
			if rule.MatchType == "regex_any" {
				re, err := regexp.Compile(pattern)
				if err != nil {
					return nil, fmt.Errorf("Error rewrite rule %d: invalid regular expression: %w", i+1, err)
				}
				next.regexps = append(next.regexps, re)
			}
			next.Patterns = append(next.Patterns, pattern)
		}
		if len(next.Patterns) == 0 {
			return nil, fmt.Errorf("Error rewrite rule %d: patterns cannot be empty", i+1)
		}
		compiled = append(compiled, next)
	}
	slices.SortStableFunc(compiled, func(a, b compiledErrorMessageRewriteRule) int {
		if priority := cmp.Compare(b.Priority, a.Priority); priority != 0 {
			return priority
		}
		return cmp.Compare(a.Key, b.Key)
	})
	return compiled, nil
}

func ValidateErrorMessageRewriteRules(raw string) error {
	_, err := compileErrorMessageRewriteRules(raw)
	return err
}

// PreviewErrorMessageRewrite uses exactly the same matching semantics as output.
func PreviewErrorMessageRewrite(message, raw string, source ErrorMessageSource) (string, error) {
	if source != ErrorMessageSourceLocal && source != ErrorMessageSourceUpstream {
		return "", fmt.Errorf("Invalid error rewrite preview source")
	}
	rules, err := compileErrorMessageRewriteRules(raw)
	if err != nil {
		return "", err
	}
	return applyErrorMessageRewrite(message, rules, source), nil
}

// RewriteErrorMessage is only used at client output boundaries. Missing source
// means a local management/API error, never an upstream-controlled payload type.
func RewriteErrorMessage(message string, sources ...ErrorMessageSource) string {
	OptionMapRWMutex.RLock()
	enabled := strings.EqualFold(OptionMap[OptionErrorMessageRewriteEnabled], "true")
	raw := OptionMap[OptionErrorMessageRewriteRules]
	OptionMapRWMutex.RUnlock()
	if !enabled || strings.TrimSpace(message) == "" {
		return message
	}
	source := ErrorMessageSourceLocal
	if len(sources) > 0 {
		source = sources[0]
	}
	errorMessageRewriteCache.Lock()
	if raw != errorMessageRewriteCache.raw {
		errorMessageRewriteCache.rules, _ = compileErrorMessageRewriteRules(raw)
		errorMessageRewriteCache.raw = raw
	}
	rules := errorMessageRewriteCache.rules
	errorMessageRewriteCache.Unlock()
	return applyErrorMessageRewrite(message, rules, source)
}

func applyErrorMessageRewrite(message string, rules []compiledErrorMessageRewriteRule, source ErrorMessageSource) string {
	suffix := requestIDSuffixRegexp.FindString(message)
	core := strings.TrimSpace(strings.TrimSuffix(message, suffix))
	if core == "" {
		return message
	}
	for _, rule := range rules {
		if !rule.Enabled || (rule.Source != ErrorMessageSourceAll && rule.Source != source) {
			continue
		}
		for i, pattern := range rule.Patterns {
			matched := false
			switch rule.MatchType {
			case "exact_any":
				matched = strings.EqualFold(core, pattern)
			case "contains_any":
				matched = strings.Contains(strings.ToLower(core), strings.ToLower(pattern))
			case "regex_any":
				matched = rule.regexps[i].MatchString(core)
			}
			if matched {
				return rule.Replace + suffix
			}
		}
	}
	return message
}
