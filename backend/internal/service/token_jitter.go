package service

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
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
	if ctx == nil {
		ctx = context.Background()
	}

	raw, err := s.settingRepo.GetValue(ctx, SettingKeyTokenJitter)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
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
	return updated, nil
}

func ApplyTokenJitter(cfg *TokenJitterConfig, inputTokens, outputTokens, cacheCreationTokens, cacheReadTokens int) (int, int, int, int) {
	if cfg == nil || !cfg.Enabled {
		return inputTokens, outputTokens, cacheCreationTokens, cacheReadTokens
	}

	// 普通 Token 浮动判断
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

	// 缓存 Token 浮动判断
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
