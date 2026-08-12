export default {
  tokenControl: {
    title: 'Token 消耗控制',
    description:
      '对上游返回的 token 数量进行随机正向微调，可用于计费压力测试或附加少量消耗浮动。',
    enableJitter: '启用 Token 抖动',
    enableJitterHint: '启用后，token 数量将按照以下设置随机向上浮动。',
    normalToken: '普通 Token',
    normalTokenHint: '作用于提示词（输入）和回复（输出）token。',
    cacheToken: '缓存 Token',
    cacheTokenHint: '作用于缓存读取和缓存创建 token。',
    range: '浮动幅度',
    probability: '触发概率',
    previewNormal: '每次请求有 {prob}% 的概率将输入/输出 token 向上浮动 0–{range}%。',
    previewCache: '每次请求有 {prob}% 的概率将缓存 token 向上浮动 0–{range}%。',
  },
}
