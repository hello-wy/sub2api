//go:build unit

package service

import "context"

// openAIHealthRepoStub captures shared credential health mutations used by
// STATE ticket and OAuth health tests.
type openAIHealthRepoStub struct {
	rateLimitAccountRepoStub
	mutation OpenAIOAuthHealthMutation
	expected *Account
	members  []*Account
}

func (r *openAIHealthRepoStub) ApplyOpenAIOAuthHealth(_ context.Context, expected *Account, mutation OpenAIOAuthHealthMutation) ([]*Account, error) {
	r.expected, r.mutation = expected, mutation
	for _, member := range r.members {
		member.RateLimitResetAt = mutation.RateLimitUntil
	}
	return r.members, nil
}

func (r *openAIHealthRepoStub) ClearOpenAIOAuthRefreshCooldown(context.Context, *Account) ([]*Account, error) {
	return nil, nil
}
