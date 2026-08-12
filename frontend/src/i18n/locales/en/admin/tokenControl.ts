export default {
  tokenControl: {
    title: 'Token Consumption Control',
    description:
      'Randomly inflate token counts reported by upstream providers. Useful for testing billing variance or adding minor consumption adjustments.',
    enableJitter: 'Enable Token Jitter',
    enableJitterHint: 'When enabled, token counts may be randomly inflated per the settings below.',
    normalToken: 'Normal Tokens',
    normalTokenHint: 'Applies to prompt (input) and completion (output) tokens.',
    cacheToken: 'Cache Tokens',
    cacheTokenHint: 'Applies to cache-read and cache-creation tokens.',
    range: 'Jitter Range',
    probability: 'Trigger Probability',
    minTokens: 'Min Token Threshold',
    minTokensHint: 'Only triggers when total tokens reach or exceed this threshold (0 = no limit).',
    previewNormal:
      'Each request has a {prob}% chance of inflating input/output tokens by 0–{range}% (threshold: >= {min} tokens).',
    previewCache:
      'Each request has a {prob}% chance of inflating cache tokens by 0–{range}% (threshold: >= {min} tokens).',
  },
}
