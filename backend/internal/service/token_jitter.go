package service

import (
	"context"
	"encoding/json"
	"errors"
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

type TokenJitterConfig struct {
	Enabled                bool    `json:"enabled"`
	NormalTokenMode        string  `json:"normal_token_mode"` // "all", "input_only", "output_only"
	NormalTokenRange       float64 `json:"normal_token_range"`
	NormalTokenProbability float64 `json:"normal_token_probability"`
	NormalTokenMinTokens   int     `json:"normal_token_min_tokens"`
	CacheTokenMode         string  `json:"cache_token_mode"` // "all", "read_only", "creation_only"
	CacheTokenRange        float64 `json:"cache_token_range"`
	CacheTokenProbability  float64 `json:"cache_token_probability"`
	CacheTokenMinTokens    int     `json:"cache_token_min_tokens"`
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
	}
}

func validateTokenJitterConfig(cfg *TokenJitterConfig) error {
	if cfg == nil {
		return errors.New("invalid config")
	}
	if cfg.NormalTokenMode != ModeAll && cfg.NormalTokenMode != ModeInputOnly && cfg.NormalTokenMode != ModeOutputOnly {
		cfg.NormalTokenMode = ModeAll
	}
	if cfg.NormalTokenRange < 0 || cfg.NormalTokenRange > 10 {
		return errors.New("normal_token_range must be between 0 and 10")
	}
	if cfg.NormalTokenProbability < 0 || cfg.NormalTokenProbability > 100 {
		return errors.New("normal_token_probability must be between 0 and 100")
	}
	if cfg.NormalTokenMinTokens < 0 {
		return errors.New("normal_token_min_tokens must be greater than or equal to 0")
	}
	if cfg.CacheTokenMode != ModeAll && cfg.CacheTokenMode != ModeReadOnly && cfg.CacheTokenMode != ModeCreationOnly {
		cfg.CacheTokenMode = ModeAll
	}
	if cfg.CacheTokenRange < 0 || cfg.CacheTokenRange > 10 {
		return errors.New("cache_token_range must be between 0 and 10")
	}
	if cfg.CacheTokenProbability < 0 || cfg.CacheTokenProbability > 100 {
		return errors.New("cache_token_probability must be between 0 and 100")
	}
	if cfg.CacheTokenMinTokens < 0 {
		return errors.New("cache_token_min_tokens must be greater than or equal to 0")
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
		if errors.Is(err, ErrSettingNotFound) {
			globalTokenJitterCache.Lock()
			globalTokenJitterCache.cfg = defaultCfg
			globalTokenJitterCache.updatedAt = time.Now()
			globalTokenJitterCache.Unlock()
			return defaultCfg, nil
		}
		return nil, err
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

func ApplyTokenJitter(cfg *TokenJitterConfig, inputTokens, outputTokens, cacheCreationTokens, cacheReadTokens int) (int, int, int, int) {
	if cfg == nil || !cfg.Enabled {
		return inputTokens, outputTokens, cacheCreationTokens, cacheReadTokens
	}

	totalNormal := inputTokens + outputTokens
	if totalNormal >= cfg.NormalTokenMinTokens && cfg.NormalTokenProbability > 0 && cfg.NormalTokenRange > 0 {
		if rand.Float64()*100 < cfg.NormalTokenProbability {
			jitterMultiplier := 1 + (rand.Float64() * cfg.NormalTokenRange / 100)
			switch cfg.NormalTokenMode {
			case ModeInputOnly:
				inputTokens = int(float64(inputTokens) * jitterMultiplier)
			case ModeOutputOnly:
				outputTokens = int(float64(outputTokens) * jitterMultiplier)
			default: // ModeAll
				inputTokens = int(float64(inputTokens) * jitterMultiplier)
				outputTokens = int(float64(outputTokens) * jitterMultiplier)
			}
		}
	}

	totalCache := cacheCreationTokens + cacheReadTokens
	if totalCache >= cfg.CacheTokenMinTokens && cfg.CacheTokenProbability > 0 && cfg.CacheTokenRange > 0 {
		if rand.Float64()*100 < cfg.CacheTokenProbability {
			jitterMultiplier := 1 + (rand.Float64() * cfg.CacheTokenRange / 100)
			switch cfg.CacheTokenMode {
			case ModeReadOnly:
				cacheReadTokens = int(float64(cacheReadTokens) * jitterMultiplier)
			case ModeCreationOnly:
				cacheCreationTokens = int(float64(cacheCreationTokens) * jitterMultiplier)
			default: // ModeAll
				cacheCreationTokens = int(float64(cacheCreationTokens) * jitterMultiplier)
				cacheReadTokens = int(float64(cacheReadTokens) * jitterMultiplier)
			}
		}
	}

	return inputTokens, outputTokens, cacheCreationTokens, cacheReadTokens
}

// RewriteJSONUsageBytes 检查 JSON 数据中是否有 usage 节点，若有且开启了抖动，则改写其中的 usage Token 数并重算 total_tokens
func RewriteJSONUsageBytes(cfg *TokenJitterConfig, body []byte) []byte {
	if cfg == nil || !cfg.Enabled || len(body) == 0 {
		return body
	}

	// 支持根节点为 usage，或包裹在 "usage" / "response.usage" 节点中的形式
	var usageRes gjson.Result
	var prefixPath string

	if res := gjson.GetBytes(body, "usage"); res.Exists() && res.IsObject() {
		usageRes = res
		prefixPath = "usage."
	} else if res := gjson.GetBytes(body, "response.usage"); res.Exists() && res.IsObject() {
		usageRes = res
		prefixPath = "response.usage."
	} else if res := gjson.GetBytes(body, "prompt_tokens"); res.Exists() {
		usageRes = gjson.ParseBytes(body)
		prefixPath = ""
	} else if res := gjson.GetBytes(body, "input_tokens"); res.Exists() {
		usageRes = gjson.ParseBytes(body)
		prefixPath = ""
	} else {
		return body
	}

	// 提取常规与缓存 Token 数（兼容 OpenAI 与 Anthropic 格式）
	inputTokens := int(usageRes.Get("prompt_tokens").Int())
	if inputTokens == 0 {
		inputTokens = int(usageRes.Get("input_tokens").Int())
	}
	outputTokens := int(usageRes.Get("completion_tokens").Int())
	if outputTokens == 0 {
		outputTokens = int(usageRes.Get("output_tokens").Int())
	}

	cacheReadTokens := int(usageRes.Get("prompt_tokens_details.cached_tokens").Int())
	if cacheReadTokens == 0 {
		cacheReadTokens = int(usageRes.Get("cache_read_input_tokens").Int())
	}
	cacheCreationTokens := int(usageRes.Get("prompt_tokens_details.cache_creation_tokens").Int())
	if cacheCreationTokens == 0 {
		cacheCreationTokens = int(usageRes.Get("cache_creation_input_tokens").Int())
	}

	// 应用抖动算法
	newInput, newOutput, newCacheCreation, newCacheRead := ApplyTokenJitter(
		cfg,
		inputTokens,
		outputTokens,
		cacheCreationTokens,
		cacheReadTokens,
	)

	// 如果没有变化，原样返回
	if newInput == inputTokens && newOutput == outputTokens &&
		newCacheCreation == cacheCreationTokens && newCacheRead == cacheReadTokens {
		return body
	}

	out := body
	var err error

	// OpenAI 语法: prompt_tokens, completion_tokens, total_tokens
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
		}
	} else if usageRes.Get("input_tokens").Exists() {
		// Anthropic / Gemini 语法: input_tokens, output_tokens
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
	}

	return out
}
