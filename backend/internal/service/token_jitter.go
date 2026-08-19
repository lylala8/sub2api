package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	ModeAll          = "all"
	ModeInputOnly    = "input_only"
	ModeOutputOnly   = "output_only"
	ModeReadOnly     = "read_only"
	ModeCreationOnly = "creation_only"
)

type GroupCacheRatio struct {
	GroupID   int64   `json:"group_id"`
	GroupName string  `json:"group_name"`
	Ratio     float64 `json:"ratio"` // 0.0 - 100.0 (百分比)
}

type TokenJitterConfig struct {
	Enabled                bool              `json:"enabled"`
	NormalTokenMode        string            `json:"normal_token_mode"` // "all", "input_only", "output_only"
	NormalTokenRange       float64           `json:"normal_token_range"`
	NormalTokenProbability float64           `json:"normal_token_probability"`
	NormalTokenMinTokens   int               `json:"normal_token_min_tokens"`
	CacheTokenMode         string            `json:"cache_token_mode"` // "all", "read_only", "creation_only"
	CacheTokenRange        float64           `json:"cache_token_range"`
	CacheTokenProbability  float64           `json:"cache_token_probability"`
	CacheTokenMinTokens    int               `json:"cache_token_min_tokens"`
	GroupCacheRatios       []GroupCacheRatio `json:"group_cache_ratios"`
}

// 本地内存缓存，避免高并发计费热路径每次去数据库读配置
type tokenJitterCache struct {
	sync.RWMutex
	cfg       *TokenJitterConfig
	updatedAt time.Time
}

var globalTokenJitterCache tokenJitterCache
const tokenJitterCacheTTL = 5 * time.Second

func defaultTokenJitterConfig() *TokenJitterConfig {
	return &TokenJitterConfig{
		Enabled:                false,
		NormalTokenMode:        ModeAll,
		NormalTokenRange:       0,
		NormalTokenProbability: 0,
		NormalTokenMinTokens:   0,
		CacheTokenMode:         ModeAll,
		CacheTokenRange:        0,
		CacheTokenProbability:  0,
		CacheTokenMinTokens:    0,
		GroupCacheRatios:       []GroupCacheRatio{},
	}
}

func validateTokenJitterConfig(cfg *TokenJitterConfig) error {
	if cfg == nil {
		return errors.New("config cannot be nil")
	}
	if cfg.NormalTokenRange < 0 || cfg.NormalTokenRange > 50 {
		return errors.New("normal_token_range must be between 0 and 50")
	}
	if cfg.NormalTokenProbability < 0 || cfg.NormalTokenProbability > 100 {
		return errors.New("normal_token_probability must be between 0 and 100")
	}
	if cfg.CacheTokenRange < 0 || cfg.CacheTokenRange > 50 {
		return errors.New("cache_token_range must be between 0 and 50")
	}
	if cfg.CacheTokenProbability < 0 || cfg.CacheTokenProbability > 100 {
		return errors.New("cache_token_probability must be between 0 and 100")
	}
	if cfg.NormalTokenMinTokens < 0 {
		return errors.New("normal_token_min_tokens must be >= 0")
	}
	if cfg.CacheTokenMinTokens < 0 {
		return errors.New("cache_token_min_tokens must be >= 0")
	}
	for _, g := range cfg.GroupCacheRatios {
		if g.Ratio < 0 || g.Ratio > 100 {
			return errors.New("group_cache_ratio must be between 0 and 100")
		}
	}
	return nil
}

func (s *OpsService) GetTokenJitterConfig(ctx context.Context) (*TokenJitterConfig, error) {
	defaultCfg := defaultTokenJitterConfig()
	if s == nil || s.settingRepo == nil {
		return defaultCfg, nil
	}

	globalTokenJitterCache.RLock()
	if globalTokenJitterCache.cfg != nil && time.Since(globalTokenJitterCache.updatedAt) < tokenJitterCacheTTL {
		cached := globalTokenJitterCache.cfg
		globalTokenJitterCache.RUnlock()
		return cached, nil
	}
	globalTokenJitterCache.RUnlock()

	if ctx == nil {
		ctx = context.Background()
	}

	raw, err := s.settingRepo.GetValue(ctx, SettingKeyTokenJitter)
	if err != nil {
		globalTokenJitterCache.Lock()
		globalTokenJitterCache.cfg = defaultCfg
		globalTokenJitterCache.updatedAt = time.Now()
		globalTokenJitterCache.Unlock()
		return defaultCfg, nil
	}

	cfg := &TokenJitterConfig{}
	if err := json.Unmarshal([]byte(raw), cfg); err != nil {
		return defaultCfg, nil
	}

	if cfg.NormalTokenMode == "" {
		cfg.NormalTokenMode = ModeAll
	}
	if cfg.CacheTokenMode == "" {
		cfg.CacheTokenMode = ModeAll
	}

	globalTokenJitterCache.Lock()
	globalTokenJitterCache.cfg = cfg
	globalTokenJitterCache.updatedAt = time.Now()
	globalTokenJitterCache.Unlock()

	return cfg, nil
}

func (s *OpsService) UpdateTokenJitterConfig(ctx context.Context, cfg *TokenJitterConfig) (*TokenJitterConfig, error) {
	if s == nil || s.settingRepo == nil {
		return nil, errors.New("setting repository not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if cfg == nil {
		return nil, errors.New("invalid config")
	}

	if err := validateTokenJitterConfig(cfg); err != nil {
		return nil, err
	}

	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	if err := s.settingRepo.Set(ctx, SettingKeyTokenJitter, string(raw)); err != nil {
		return nil, err
	}

	updated := &TokenJitterConfig{}
	_ = json.Unmarshal(raw, updated)

	globalTokenJitterCache.Lock()
	globalTokenJitterCache.cfg = updated
	globalTokenJitterCache.updatedAt = time.Now()
	globalTokenJitterCache.Unlock()

	return updated, nil
}

func ApplyGroupCacheRatio(cfg *TokenJitterConfig, groupID int64, inputTokens, cacheReadTokens int) (int, int) {
	if cfg == nil || !cfg.Enabled || groupID <= 0 || cacheReadTokens <= 0 || len(cfg.GroupCacheRatios) == 0 {
		return inputTokens, cacheReadTokens
	}

	var matchedRatio *float64
	for _, g := range cfg.GroupCacheRatios {
		if g.GroupID == groupID {
			r := g.Ratio
			matchedRatio = &r
			break
		}
	}

	// 未配置该分组（不选不动它），保持 100% 默认不变
	if matchedRatio == nil || *matchedRatio >= 100.0 {
		return inputTokens, cacheReadTokens
	}

	ratio := *matchedRatio
	if ratio < 0 {
		ratio = 0
	}

	// 例如 80% 比例: 1000 缓存 -> 保留 800 缓存，剩下的 200 转移到常规 Input Token 正常计费
	keptCacheRead := int(math.Floor(float64(cacheReadTokens) * (ratio / 100.0)))
	transferredToInput := cacheReadTokens - keptCacheRead

	if transferredToInput > 0 {
		slog.Info("token_jitter.group_cache_ratio_applied",
			"group_id", groupID,
			"ratio", ratio,
			"orig_input_tokens", inputTokens,
			"orig_cache_read_tokens", cacheReadTokens,
			"new_input_tokens", inputTokens+transferredToInput,
			"new_cache_read_tokens", keptCacheRead,
			"transferred_to_input", transferredToInput,
		)
	}

	return inputTokens + transferredToInput, keptCacheRead
}

func ApplyTokenJitter(cfg *TokenJitterConfig, inputTokens, outputTokens, cacheCreationTokens, cacheReadTokens int, groupID ...int64) (int, int, int, int) {
	if cfg == nil || !cfg.Enabled {
		return inputTokens, outputTokens, cacheCreationTokens, cacheReadTokens
	}

	var gID int64
	if len(groupID) > 0 {
		gID = groupID[0]
	}

	// 1. 若匹配分组配置，优先执行缓存 Token 转化拆分
	if gID > 0 && cacheReadTokens > 0 {
		inputTokens, cacheReadTokens = ApplyGroupCacheRatio(cfg, gID, inputTokens, cacheReadTokens)
	}

	// 2. 执行 Token 随机向上抖动
	origInput := inputTokens
	origOutput := outputTokens
	totalNormal := inputTokens + outputTokens
	if totalNormal >= cfg.NormalTokenMinTokens && cfg.NormalTokenProbability > 0 && cfg.NormalTokenRange > 0 {
		if rand.Float64()*100 < cfg.NormalTokenProbability {
			jitterMultiplier := 1 + (rand.Float64() * cfg.NormalTokenRange / 100)
			switch cfg.NormalTokenMode {
			case ModeInputOnly:
				inputTokens = int(math.Ceil(float64(inputTokens) * jitterMultiplier))
			case ModeOutputOnly:
				outputTokens = int(math.Ceil(float64(outputTokens) * jitterMultiplier))
			default: // ModeAll
				inputTokens = int(math.Ceil(float64(inputTokens) * jitterMultiplier))
				outputTokens = int(math.Ceil(float64(outputTokens) * jitterMultiplier))
			}
			slog.Info("token_jitter.normal_jitter_applied",
				"group_id", gID,
				"mode", cfg.NormalTokenMode,
				"orig_input_tokens", origInput,
				"orig_output_tokens", origOutput,
				"new_input_tokens", inputTokens,
				"new_output_tokens", outputTokens,
				"input_added", inputTokens-origInput,
				"output_added", outputTokens-origOutput,
			)
		}
	}

	origCacheCreation := cacheCreationTokens
	origCacheRead := cacheReadTokens
	totalCache := cacheCreationTokens + cacheReadTokens
	if totalCache >= cfg.CacheTokenMinTokens && cfg.CacheTokenProbability > 0 && cfg.CacheTokenRange > 0 {
		if rand.Float64()*100 < cfg.CacheTokenProbability {
			jitterMultiplier := 1 + (rand.Float64() * cfg.CacheTokenRange / 100)
			switch cfg.CacheTokenMode {
			case ModeReadOnly:
				cacheReadTokens = int(math.Ceil(float64(cacheReadTokens) * jitterMultiplier))
			case ModeCreationOnly:
				cacheCreationTokens = int(math.Ceil(float64(cacheCreationTokens) * jitterMultiplier))
			default: // ModeAll
				cacheCreationTokens = int(math.Ceil(float64(cacheCreationTokens) * jitterMultiplier))
				cacheReadTokens = int(math.Ceil(float64(cacheReadTokens) * jitterMultiplier))
			}
			slog.Info("token_jitter.cache_jitter_applied",
				"group_id", gID,
				"mode", cfg.CacheTokenMode,
				"orig_cache_creation_tokens", origCacheCreation,
				"orig_cache_read_tokens", origCacheRead,
				"new_cache_creation_tokens", cacheCreationTokens,
				"new_cache_read_tokens", cacheReadTokens,
				"cache_creation_added", cacheCreationTokens-origCacheCreation,
				"cache_read_added", cacheReadTokens-origCacheRead,
			)
		}
	}

	return inputTokens, outputTokens, cacheCreationTokens, cacheReadTokens
}

// RewriteJSONUsageBytes 检查 JSON 数据中是否有 usage 节点，若有且开启了抖动，则改写其中的 usage Token 数并重算 total_tokens
func RewriteJSONUsageBytes(cfg *TokenJitterConfig, body []byte, groupID ...int64) (outBytes []byte) {
	// 防御性 panic 恢复：如果解析/改写遇到任何未预期异常，静默恢复并绝对保证返回原始 body，不影响 API 主流程
	defer func() {
		if r := recover(); r != nil {
			outBytes = body
		}
	}()

	if cfg == nil || !cfg.Enabled || len(body) == 0 {
		return body
	}

	var gID int64
	if len(groupID) > 0 {
		gID = groupID[0]
	}

	// 支持根节点为 usage / response.usage / usageMetadata，或无包裹形式
	var usageRes gjson.Result
	var prefixPath string

	if res := gjson.GetBytes(body, "usage"); res.Exists() && res.IsObject() {
		usageRes = res
		prefixPath = "usage."
	} else if res := gjson.GetBytes(body, "response.usage"); res.Exists() && res.IsObject() {
		usageRes = res
		prefixPath = "response.usage."
	} else if res := gjson.GetBytes(body, "usageMetadata"); res.Exists() && res.IsObject() {
		usageRes = res
		prefixPath = "usageMetadata."
	} else if res := gjson.GetBytes(body, "prompt_tokens"); res.Exists() {
		usageRes = gjson.ParseBytes(body)
		prefixPath = ""
	} else if res := gjson.GetBytes(body, "promptTokenCount"); res.Exists() {
		usageRes = gjson.ParseBytes(body)
		prefixPath = ""
	} else if res := gjson.GetBytes(body, "input_tokens"); res.Exists() {
		usageRes = gjson.ParseBytes(body)
		prefixPath = ""
	} else {
		return body
	}

	// 提取常规与缓存 Token 数（兼容 OpenAI、Anthropic、Gemini、DeepSeek 格式）
	inputTokens := int(usageRes.Get("prompt_tokens").Int())
	if inputTokens == 0 {
		inputTokens = int(usageRes.Get("input_tokens").Int())
	}
	if inputTokens == 0 {
		inputTokens = int(usageRes.Get("promptTokenCount").Int())
	}

	outputTokens := int(usageRes.Get("completion_tokens").Int())
	if outputTokens == 0 {
		outputTokens = int(usageRes.Get("output_tokens").Int())
	}
	if outputTokens == 0 {
		outputTokens = int(usageRes.Get("candidatesTokenCount").Int())
	}

	cacheReadTokens := int(usageRes.Get("prompt_tokens_details.cached_tokens").Int())
	if cacheReadTokens == 0 {
		cacheReadTokens = int(usageRes.Get("cache_read_input_tokens").Int())
	}
	if cacheReadTokens == 0 {
		cacheReadTokens = int(usageRes.Get("cachedContentTokenCount").Int())
	}
	if cacheReadTokens == 0 {
		cacheReadTokens = int(usageRes.Get("prompt_cache_hit_tokens").Int())
	}

	cacheCreationTokens := int(usageRes.Get("prompt_tokens_details.cache_creation_tokens").Int())
	if cacheCreationTokens == 0 {
		cacheCreationTokens = int(usageRes.Get("cache_creation_input_tokens").Int())
	}
	if cacheCreationTokens == 0 {
		cacheCreationTokens = int(usageRes.Get("prompt_cache_miss_tokens").Int())
	}

	// 应用分组缓存转化与抖动算法
	newInput, newOutput, newCacheCreation, newCacheRead := ApplyTokenJitter(
		cfg,
		inputTokens,
		outputTokens,
		cacheCreationTokens,
		cacheReadTokens,
		gID,
	)

	// 如果没有变化，原样返回
	if newInput == inputTokens && newOutput == outputTokens &&
		newCacheCreation == cacheCreationTokens && newCacheRead == cacheReadTokens {
		return body
	}

	out := body
	var err error

	// 1. OpenAI 语法: prompt_tokens, completion_tokens, total_tokens
	if usageRes.Get("prompt_tokens").Exists() {
		out, err = sjson.SetBytes(out, prefixPath+"prompt_tokens", newInput)
		if err == nil {
			out, _ = sjson.SetBytes(out, prefixPath+"completion_tokens", newOutput)
			out, _ = sjson.SetBytes(out, prefixPath+"total_tokens", newInput+newOutput)
			if usageRes.Get("prompt_tokens_details.cached_tokens").Exists() {
				out, _ = sjson.SetBytes(out, prefixPath+"prompt_tokens_details.cached_tokens", newCacheRead)
			}
			if usageRes.Get("prompt_tokens_details.cache_creation_tokens").Exists() {
				out, _ = sjson.SetBytes(out, prefixPath+"prompt_tokens_details.cache_creation_tokens", newCacheCreation)
			}
			if usageRes.Get("prompt_cache_hit_tokens").Exists() {
				out, _ = sjson.SetBytes(out, prefixPath+"prompt_cache_hit_tokens", newCacheRead)
			}
		}
	} else if usageRes.Get("input_tokens").Exists() {
		// 2. Anthropic 语法: input_tokens, output_tokens
		out, err = sjson.SetBytes(out, prefixPath+"input_tokens", newInput)
		if err == nil {
			out, _ = sjson.SetBytes(out, prefixPath+"output_tokens", newOutput)
			if usageRes.Get("cache_read_input_tokens").Exists() {
				out, _ = sjson.SetBytes(out, prefixPath+"cache_read_input_tokens", newCacheRead)
			}
			if usageRes.Get("cache_creation_input_tokens").Exists() {
				out, _ = sjson.SetBytes(out, prefixPath+"cache_creation_input_tokens", newCacheCreation)
			}
		}
	} else if usageRes.Get("promptTokenCount").Exists() {
		// 3. Gemini 语法: promptTokenCount, candidatesTokenCount, totalTokenCount
		out, err = sjson.SetBytes(out, prefixPath+"promptTokenCount", newInput)
		if err == nil {
			out, _ = sjson.SetBytes(out, prefixPath+"candidatesTokenCount", newOutput)
			out, _ = sjson.SetBytes(out, prefixPath+"totalTokenCount", newInput+newOutput)
			if usageRes.Get("cachedContentTokenCount").Exists() {
				out, _ = sjson.SetBytes(out, prefixPath+"cachedContentTokenCount", newCacheRead)
			}
		}
	}

	return EnsureOpenAICacheDetailsCompat(out)
}

// EnsureOpenAICacheDetailsCompat 保证 /v1/responses 协议的缓存字段能够兼容下游 NewAPI / 老客户端
func EnsureOpenAICacheDetailsCompat(body []byte) []byte {
	if len(body) == 0 {
		return body
	}

	hasSSEPrefix := bytes.HasPrefix(body, []byte("data: "))
	jsonBytes := body
	if hasSSEPrefix {
		jsonBytes = bytes.TrimPrefix(body, []byte("data: "))
	}

	var prefixPath string
	var usageRes gjson.Result

	if res := gjson.GetBytes(jsonBytes, "usage"); res.Exists() && res.IsObject() {
		usageRes = res
		prefixPath = "usage."
	} else if res := gjson.GetBytes(jsonBytes, "response.usage"); res.Exists() && res.IsObject() {
		usageRes = res
		prefixPath = "response.usage."
	} else if res := gjson.GetBytes(jsonBytes, "input_tokens_details"); res.Exists() {
		usageRes = gjson.ParseBytes(jsonBytes)
		prefixPath = ""
	} else {
		return body
	}

	// 如果存在 input_tokens_details.cached_tokens，补齐经典的 prompt_tokens_details.cached_tokens 以及 cached_tokens
	cached := usageRes.Get("input_tokens_details.cached_tokens").Int()
	if cached == 0 {
		cached = usageRes.Get("prompt_tokens_details.cached_tokens").Int()
	}

	if cached > 0 {
		out := jsonBytes
		var modified bool

		if !usageRes.Get("prompt_tokens_details.cached_tokens").Exists() {
			if updated, err := sjson.SetBytes(out, prefixPath+"prompt_tokens_details.cached_tokens", cached); err == nil {
				out = updated
				modified = true
			}
		}
		if !usageRes.Get("cached_tokens").Exists() {
			if updated, err := sjson.SetBytes(out, prefixPath+"cached_tokens", cached); err == nil {
				out = updated
				modified = true
			}
		}
		if modified {
			if hasSSEPrefix {
				return append([]byte("data: "), out...)
			}
			return out
		}
	}
	return body
}
