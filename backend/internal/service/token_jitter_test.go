package service

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateTokenJitterConfig(t *testing.T) {
	cfg := &TokenJitterConfig{
		Enabled:                true,
		NormalTokenMode:        ModeAll,
		NormalTokenRange:       5.0,
		NormalTokenProbability: 50.0,
		NormalTokenMinTokens:   100,
		CacheTokenMode:         ModeReadOnly,
		CacheTokenRange:        3.0,
		CacheTokenProbability:  20.0,
		CacheTokenMinTokens:    50,
	}
	require.NoError(t, validateTokenJitterConfig(cfg))

	// Invalid NormalTokenRange (> 10)
	invalidCfg := *cfg
	invalidCfg.NormalTokenRange = 15.0
	require.Error(t, validateTokenJitterConfig(&invalidCfg))

	// Invalid CacheTokenProbability (> 100)
	invalidCfg = *cfg
	invalidCfg.CacheTokenProbability = 150.0
	require.Error(t, validateTokenJitterConfig(&invalidCfg))
}

func TestApplyTokenJitter_UpwardOnly(t *testing.T) {
	cfg := &TokenJitterConfig{
		Enabled:                true,
		NormalTokenMode:        ModeAll,
		NormalTokenRange:       10.0, // max 10%
		NormalTokenProbability: 100.0, // 100% trigger
		NormalTokenMinTokens:   0,
		CacheTokenMode:         ModeAll,
		CacheTokenRange:        10.0,
		CacheTokenProbability:  100.0,
		CacheTokenMinTokens:    0,
	}

	input, output, cacheCreation, cacheRead := 100, 200, 50, 80
	nin, nout, ncc, ncr := ApplyTokenJitter(cfg, input, output, cacheCreation, cacheRead)

	// Must be strictly greater than or equal to original values
	assert.GreaterOrEqual(t, nin, input)
	assert.GreaterOrEqual(t, nout, output)
	assert.GreaterOrEqual(t, ncc, cacheCreation)
	assert.GreaterOrEqual(t, ncr, cacheRead)

	// Must not exceed +10% + 1 (math.Ceil bound)
	assert.LessOrEqual(t, nin, int(math.Ceil(float64(input)*1.10)))
	assert.LessOrEqual(t, nout, int(math.Ceil(float64(output)*1.10)))
}

func TestApplyTokenJitter_Disabled(t *testing.T) {
	cfg := &TokenJitterConfig{
		Enabled: false,
	}
	nin, nout, ncc, ncr := ApplyTokenJitter(cfg, 100, 200, 50, 80)
	assert.Equal(t, 100, nin)
	assert.Equal(t, 200, nout)
	assert.Equal(t, 50, ncc)
	assert.Equal(t, 80, ncr)
}

func TestRewriteJSONUsageBytes_OpenAIFormat(t *testing.T) {
	cfg := &TokenJitterConfig{
		Enabled:                true,
		NormalTokenMode:        ModeAll,
		NormalTokenRange:       10.0,
		NormalTokenProbability: 100.0,
		NormalTokenMinTokens:   0,
	}

	rawJSON := []byte(`{"id":"chatcmpl-123","usage":{"prompt_tokens":100,"completion_tokens":200,"total_tokens":300}}`)
	out := RewriteJSONUsageBytes(cfg, rawJSON)

	require.NotEmpty(t, out)
	assert.NotEqual(t, string(rawJSON), string(out))
}
