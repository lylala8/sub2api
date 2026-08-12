import apiClient from './index'

export interface TokenJitterConfig {
  enabled: boolean
  normal_token_range: number
  normal_token_probability: number
  cache_token_range: number
  cache_token_probability: number
}

export async function getTokenJitterConfig(): Promise<TokenJitterConfig> {
  const response = await apiClient.get<TokenJitterConfig>('/api/admin/settings/token-jitter')
  return response.data
}

export async function updateTokenJitterConfig(cfg: TokenJitterConfig): Promise<TokenJitterConfig> {
  const response = await apiClient.put<TokenJitterConfig>('/api/admin/settings/token-jitter', cfg)
  return response.data
}
