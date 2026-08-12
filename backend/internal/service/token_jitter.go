package service

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
)

type TokenJitterConfig struct {
	Enabled                  bool    `json:"enabled"`
	NormalTokenRange         float64 `json:"normal_token_range"`
	NormalTokenProbability   float64 `json:"normal_token_probability"`
	CacheTokenRange          float64 `json:"cache_token_range"`
	CacheTokenProbability    float64 `json:"cache_token_probability"`
}

func defaultTokenJitterConfig() *TokenJitterConfig {
	return &TokenJitterConfig{
		Enabled:                  false,
		NormalTokenRange:         0,
		NormalTokenProbability:   0,
		CacheTokenRange:          0,
		CacheTokenProbability:    0,
	}
}

func validateTokenJitterConfig(cfg *TokenJitterConfig) error {
	if cfg == nil {
		return errors.New("invalid config")
	}
	if cfg.NormalTokenRange < 0 || cfg.NormalTokenRange > 10 {
		return errors.New("normal_token_range must be between 0 and 10")
	}
	if cfg.NormalTokenProbability < 0 || cfg.NormalTokenProbability > 100 {
		return errors.New("normal_token_probability must be between 0 and 100")
	}
	if cfg.CacheTokenRange < 0 || cfg.CacheTokenRange > 10 {
		return errors.New("cache_token_range must be between 0 and 10")
	}
	if cfg.CacheTokenProbability < 0 || cfg.CacheTokenProbability > 100 {
		return errors.New("cache_token_probability must be between 0 and 100")
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

	if cfg.NormalTokenProbability > 0 && cfg.NormalTokenRange > 0 {
		if rand.Float64()*100 < cfg.NormalTokenProbability {
			jitterMultiplier := 1 + (rand.Float64() * cfg.NormalTokenRange / 100)
			inputTokens = int(float64(inputTokens) * jitterMultiplier)
			outputTokens = int(float64(outputTokens) * jitterMultiplier)
		}
	}

	if cfg.CacheTokenProbability > 0 && cfg.CacheTokenRange > 0 {
		if rand.Float64()*100 < cfg.CacheTokenProbability {
			jitterMultiplier := 1 + (rand.Float64() * cfg.CacheTokenRange / 100)
			cacheCreationTokens = int(float64(cacheCreationTokens) * jitterMultiplier)
			cacheReadTokens = int(float64(cacheReadTokens) * jitterMultiplier)
		}
	}

	return inputTokens, outputTokens, cacheCreationTokens, cacheReadTokens
}
