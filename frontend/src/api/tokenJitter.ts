import apiClient from './index'

export interface GroupCacheRatio {
  group_id: number
  group_name: string
  ratio: number
}

export interface TokenJitterConfig {
  enabled: boolean
  normal_token_mode: 'all' | 'input_only' | 'output_only'
  normal_token_range: number
  normal_token_probability: number
  normal_token_min_tokens: number
  cache_token_mode: 'all' | 'read_only' | 'creation_only'
  cache_token_range: number
  cache_token_probability: number
  cache_token_min_tokens: number
  group_cache_ratios?: GroupCacheRatio[]
}

export async function getTokenJitterConfig(): Promise<TokenJitterConfig> {
  const response = await apiClient.get<TokenJitterConfig>('/admin/settings/token-jitter')
  return response.data
}

export async function updateTokenJitterConfig(cfg: TokenJitterConfig): Promise<TokenJitterConfig> {
  const response = await apiClient.put<TokenJitterConfig>('/admin/settings/token-jitter', cfg)
  return response.data
}
