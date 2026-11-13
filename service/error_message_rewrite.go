package service

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/types"
)

// ClientOpenAIError rewrites only the serialized client message, leaving the
// original error available for retries, channel health decisions and audit logs.
func ClientOpenAIError(err *types.NewAPIError, upstreamURLs ...string) types.OpenAIError {
	result := err.ToOpenAIError(upstreamURLs...)
	result.Message = common.RewriteErrorMessage(result.Message, err.GetErrorSource())
	return result
}

func ClientClaudeError(err *types.NewAPIError, upstreamURLs ...string) types.ClaudeError {
	result := err.ToClaudeError(upstreamURLs...)
	result.Message = common.RewriteErrorMessage(result.Message, err.GetErrorSource())
	return result
}
