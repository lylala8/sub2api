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

	// Invalid NormalTokenRange (> 50)
	invalidCfg := *cfg
	invalidCfg.NormalTokenRange = 55.0
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

func TestApplyTokenJitter_StatisticalProbability(t *testing.T) {
	// 验证 1% 概率在大样本 (100,000 次) 下的统计精确度
	cfg := &TokenJitterConfig{
		Enabled:                true,
		NormalTokenMode:        ModeAll,
		NormalTokenRange:       5.0,
		NormalTokenProbability: 1.0, // 1%
		NormalTokenMinTokens:   0,
	}

	triggeredCount := 0
	iterations := 100000

	for i := 0; i < iterations; i++ {
		nin, _, _, _ := ApplyTokenJitter(cfg, 100, 100, 0, 0)
		if nin > 100 {
			triggeredCount++
		}
	}

	// 100,000 次实验中，1% 概率期望触发约为 1000 次 (允许 0.8% - 1.2% 的合理统计标准差卡方区间)
	ratio := float64(triggeredCount) / float64(iterations) * 100.0
	t.Logf("1%% probability statistical result: %d / %d = %.3f%%", triggeredCount, iterations, ratio)

	assert.GreaterOrEqual(t, ratio, 0.7)
	assert.LessOrEqual(t, ratio, 1.3)
}

func TestApplyTokenJitter_MinTokensThreshold(t *testing.T) {
	cfg := &TokenJitterConfig{
		Enabled:                true,
		NormalTokenMode:        ModeAll,
		NormalTokenRange:       50.0,   // +50%
		NormalTokenProbability: 100.0,  // 100% trigger when threshold met
		NormalTokenMinTokens:   1000,   // 必须 >= 1000 才会触发
		CacheTokenMode:         ModeAll,
		CacheTokenRange:        50.0,
		CacheTokenProbability:  100.0,
		CacheTokenMinTokens:    500,    // 必须 >= 500 才会触发
	}

	// 场景 1: 常规 Token = 800 (400+400 < 1000 阈值)，未达阈值，绝对不应该发生膨胀
	nin, nout, _, _ := ApplyTokenJitter(cfg, 400, 400, 0, 0)
	assert.Equal(t, 400, nin, "Below NormalTokenMinTokens threshold, must NOT jitter input")
	assert.Equal(t, 400, nout, "Below NormalTokenMinTokens threshold, must NOT jitter output")

	// 场景 2: 常规 Token = 1000 (500+500 >= 1000 阈值)，达到阈值，应该在 (500, 750] 随机区间内发生向上膨胀
	nin, nout, _, _ = ApplyTokenJitter(cfg, 500, 500, 0, 0)
	assert.Greater(t, nin, 500, "At NormalTokenMinTokens threshold, input MUST jitter upward (> 500)")
	assert.LessOrEqual(t, nin, 750, "At NormalTokenMinTokens threshold, input MUST NOT exceed 500 + 50%")
	assert.Greater(t, nout, 500, "At NormalTokenMinTokens threshold, output MUST jitter upward (> 500)")
	assert.LessOrEqual(t, nout, 750, "At NormalTokenMinTokens threshold, output MUST NOT exceed 500 + 50%")

	// 场景 3: 缓存 Token = 300 (< 500 阈值)，未达阈值，绝对不应该发生膨胀
	_, _, ncc, ncr := ApplyTokenJitter(cfg, 0, 0, 150, 150)
	assert.Equal(t, 150, ncc, "Below CacheTokenMinTokens threshold, must NOT jitter cache creation")
	assert.Equal(t, 150, ncr, "Below CacheTokenMinTokens threshold, must NOT jitter cache read")

	// 场景 4: 缓存 Token = 600 (300+300 >= 500 阈值)，达到阈值，应该在 (300, 450] 随机区间内发生向上膨胀
	_, _, ncc, ncr = ApplyTokenJitter(cfg, 0, 0, 300, 300)
	assert.Greater(t, ncc, 300, "At CacheTokenMinTokens threshold, cache creation MUST jitter upward (> 300)")
	assert.LessOrEqual(t, ncc, 450, "At CacheTokenMinTokens threshold, cache creation MUST NOT exceed 300 + 50%")
	assert.Greater(t, ncr, 300, "At CacheTokenMinTokens threshold, cache read MUST jitter upward (> 300)")
	assert.LessOrEqual(t, ncr, 450, "At CacheTokenMinTokens threshold, cache read MUST NOT exceed 300 + 50%")
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

func TestRewriteJSONUsageBytes_Protocols(t *testing.T) {
	cfg := &TokenJitterConfig{
		Enabled: true,
		GroupCacheRatios: []GroupCacheRatio{
			{GroupID: 1, Ratio: 80.0},
		},
	}

	// 1. OpenAI 格式 (prompt_tokens_details.cached_tokens)
	openAIJSON := []byte(`{"usage":{"prompt_tokens":1000,"completion_tokens":200,"prompt_tokens_details":{"cached_tokens":1000}}}`)
	outOpenAI := RewriteJSONUsageBytes(cfg, openAIJSON, 1)
	assert.Contains(t, string(outOpenAI), `"cached_tokens":800`)
	assert.Contains(t, string(outOpenAI), `"prompt_tokens":1200`)

	// 2. Anthropic 格式 (input_tokens & cache_read_input_tokens)
	anthropicJSON := []byte(`{"usage":{"input_tokens":500,"output_tokens":100,"cache_read_input_tokens":1000}}`)
	outAnthropic := RewriteJSONUsageBytes(cfg, anthropicJSON, 1)
	assert.Contains(t, string(outAnthropic), `"cache_read_input_tokens":800`)
	assert.Contains(t, string(outAnthropic), `"input_tokens":700`)

	// 3. Gemini 格式 (usageMetadata)
	geminiJSON := []byte(`{"usageMetadata":{"promptTokenCount":600,"candidatesTokenCount":150,"cachedContentTokenCount":1000}}`)
	outGemini := RewriteJSONUsageBytes(cfg, geminiJSON, 1)
	assert.Contains(t, string(outGemini), `"cachedContentTokenCount":800`)
	assert.Contains(t, string(outGemini), `"promptTokenCount":800`)
}

func TestApplyGroupCacheRatio(t *testing.T) {
	cfg := &TokenJitterConfig{
		Enabled: true,
		GroupCacheRatios: []GroupCacheRatio{
			{
				GroupID:   10,
				GroupName: "VIP 分组",
				Ratio:     80.0, // 80%
			},
			{
				GroupID:   20,
				GroupName: "全额计费分组",
				Ratio:     0.0, // 0%
			},
		},
	}

	// 1. 测试分组 10 (80% 比例): 1000 缓存读 Token，输入 100
	// 期待: 缓存 Token = 1000 * 80% = 800；转移 200 到输入 Token -> 输入 Token = 100 + 200 = 300
	nin, ncr := ApplyGroupCacheRatio(cfg, 10, 100, 1000)
	assert.Equal(t, 800, ncr, "80% ratio should keep 800 cache read tokens")
	assert.Equal(t, 300, nin, "80% ratio should convert 200 cache tokens to input tokens")

	// 2. 测试分组 20 (0% 比例): 1000 缓存读 Token，输入 500
	// 期待: 缓存 Token = 0；转移 1000 到输入 Token -> 输入 Token = 500 + 1000 = 1500
	nin, ncr = ApplyGroupCacheRatio(cfg, 20, 500, 1000)
	assert.Equal(t, 0, ncr, "0% ratio should keep 0 cache read tokens")
	assert.Equal(t, 1500, nin, "0% ratio should convert all 1000 cache tokens to input tokens")

	// 3. 测试未配置的分组 99 ("不选不动它"): 1000 缓存读 Token，输入 200
	// 期待: 保持 100% 不变 -> 缓存 1000，输入 200
	nin, ncr = ApplyGroupCacheRatio(cfg, 99, 200, 1000)
	assert.Equal(t, 1000, ncr, "Unconfigured group must stay 100% untouched")
	assert.Equal(t, 200, nin, "Unconfigured group input tokens must stay untouched")
}

func TestRewriteJSONUsageBytes_PanicRecovery(t *testing.T) {
	cfg := &TokenJitterConfig{
		Enabled:                true,
		NormalTokenMode:        ModeAll,
		NormalTokenRange:       10.0,
		NormalTokenProbability: 100.0,
	}

	// 传入非法/畸形数据，验证全局 panic 捕获防线，确保绝不发生崩溃且无痛返回原始输入
	invalidData := []byte(`{invalid_json_bytes`)
	require.NotPanics(t, func() {
		out := RewriteJSONUsageBytes(cfg, invalidData)
		assert.Equal(t, string(invalidData), string(out))
	})
}
