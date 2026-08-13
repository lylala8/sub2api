<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { getTokenJitterConfig, updateTokenJitterConfig, type TokenJitterConfig, type GroupCacheRatio } from '@/api/tokenJitter'
import { getAll as getAllGroups } from '@/api/admin/groups'
import type { AdminGroup } from '@/types'
import { useAppStore } from '@/stores'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const saving = ref(false)

const allGroups = ref<AdminGroup[]>([])
const selectedGroupToAdd = ref<number | ''>('')

const config = ref<TokenJitterConfig>({
  enabled: false,
  normal_token_mode: 'all',
  normal_token_range: 0,
  normal_token_probability: 0,
  normal_token_min_tokens: 0,
  cache_token_mode: 'all',
  cache_token_range: 0,
  cache_token_probability: 0,
  cache_token_min_tokens: 0,
  group_cache_ratios: [],
})

const availableGroupsForSelect = computed(() => {
  const existingIds = new Set((config.value.group_cache_ratios || []).map((g) => g.group_id))
  return allGroups.value.filter((g) => !existingIds.has(g.id))
})

const normalTargetText = computed(() => {
  switch (config.value.normal_token_mode) {
    case 'input_only':
      return t('admin.tokenControl.targetInputOnly')
    case 'output_only':
      return t('admin.tokenControl.targetOutputOnly')
    default:
      return t('admin.tokenControl.targetAllNormal')
  }
})

const cacheTargetText = computed(() => {
  switch (config.value.cache_token_mode) {
    case 'read_only':
      return t('admin.tokenControl.targetReadOnly')
    case 'creation_only':
      return t('admin.tokenControl.targetCreationOnly')
    default:
      return t('admin.tokenControl.targetAllCache')
  }
})

function addGroupRatioRule() {
  if (!selectedGroupToAdd.value) return
  const g = allGroups.value.find((item) => item.id === selectedGroupToAdd.value)
  if (!g) return

  if (!config.value.group_cache_ratios) {
    config.value.group_cache_ratios = []
  }

  config.value.group_cache_ratios.push({
    group_id: g.id,
    group_name: g.name,
    ratio: 80.0, // 默认 80%
  })

  selectedGroupToAdd.value = ''
}

function removeGroupRatioRule(index: number) {
  if (config.value.group_cache_ratios) {
    config.value.group_cache_ratios.splice(index, 1)
  }
}

async function fetchConfig() {
  loading.value = true
  try {
    const [cfg, groups] = await Promise.all([
      getTokenJitterConfig(),
      getAllGroups().catch(() => []),
    ])
    if (!cfg.group_cache_ratios) {
      cfg.group_cache_ratios = []
    }
    config.value = cfg
    allGroups.value = groups
  } catch (err) {
    appStore.showError(t('common.fetchFailed'))
  } finally {
    loading.value = false
  }
}

async function saveConfig() {
  saving.value = true
  try {
    config.value = await updateTokenJitterConfig(config.value)
    if (!config.value.group_cache_ratios) {
      config.value.group_cache_ratios = []
    }
    appStore.showSuccess(t('common.saved'))
  } catch (err: any) {
    const msg = err?.response?.data?.message || t('common.saveFailed')
    appStore.showError(msg)
  } finally {
    saving.value = false
  }
}

onMounted(fetchConfig)
</script>

<template>
  <div class="mx-auto max-w-2xl px-4 py-8 sm:px-6">
    <!-- Page Header -->
    <div class="mb-8">
      <h1 class="text-2xl font-semibold tracking-tight text-gray-900 dark:text-white">
        {{ t('admin.tokenControl.title') }}
      </h1>
      <p class="mt-1.5 text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.tokenControl.description') }}
      </p>
    </div>

    <!-- Loading skeleton -->
    <div v-if="loading" class="space-y-4">
      <div v-for="i in 3" :key="i" class="h-24 animate-pulse rounded-xl bg-gray-100 dark:bg-dark-700" />
    </div>

    <template v-else>
      <!-- Master switch -->
      <div class="mb-6 flex items-center justify-between rounded-xl border border-gray-200 bg-white px-5 py-4 shadow-sm dark:border-dark-600 dark:bg-dark-800">
        <div>
          <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.tokenControl.enableJitter') }}</p>
          <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.tokenControl.enableJitterHint') }}</p>
        </div>
        <button
          type="button"
          role="switch"
          :aria-checked="config.enabled"
          class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2"
          :class="config.enabled ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'"
          @click="config.enabled = !config.enabled"
        >
          <span
            class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
            :class="config.enabled ? 'translate-x-5' : 'translate-x-0'"
          />
        </button>
      </div>

      <!-- Controls -->
      <div class="space-y-4" :class="{ 'pointer-events-none opacity-50': !config.enabled }">
        <!-- Normal token card -->
        <div class="rounded-xl border border-gray-200 bg-white shadow-sm dark:border-dark-600 dark:bg-dark-800">
          <div class="border-b border-gray-100 px-5 py-3.5 dark:border-dark-700">
            <h2 class="text-sm font-semibold text-gray-800 dark:text-gray-100">
              {{ t('admin.tokenControl.normalToken') }}
            </h2>
            <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.tokenControl.normalTokenHint') }}</p>
          </div>
          <div class="grid grid-cols-2 gap-x-6 gap-y-4 px-5 py-4">
            <!-- Mode selection -->
            <div class="col-span-2 pb-1">
              <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                {{ t('admin.tokenControl.mode') }}
              </label>
              <select
                v-model="config.normal_token_mode"
                class="w-full rounded-lg border border-gray-300 bg-white px-3 py-1.5 text-sm text-gray-900 focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500 dark:border-dark-600 dark:bg-dark-700 dark:text-white"
              >
                <option value="all">{{ t('admin.tokenControl.modeAll') }}</option>
                <option value="input_only">{{ t('admin.tokenControl.modeInputOnly') }}</option>
                <option value="output_only">{{ t('admin.tokenControl.modeOutputOnly') }}</option>
              </select>
            </div>
            <!-- Range -->
            <div>
              <label class="mb-1.5 block text-xs font-medium text-gray-600 dark:text-gray-400">
                {{ t('admin.tokenControl.range') }}
                <span class="ml-1 font-semibold text-gray-900 dark:text-white">{{ config.normal_token_range.toFixed(1) }}%</span>
              </label>
              <input
                v-model.number="config.normal_token_range"
                type="range"
                min="0"
                max="50"
                step="0.5"
                class="range-slider w-full"
              />
              <div class="mt-1 flex justify-between text-[10px] text-gray-400">
                <span>0%</span><span>25%</span><span>50%</span>
              </div>
            </div>
            <!-- Probability -->
            <div>
              <label class="mb-1.5 block text-xs font-medium text-gray-600 dark:text-gray-400">
                {{ t('admin.tokenControl.probability') }}
                <span class="ml-1 font-semibold text-gray-900 dark:text-white">{{ config.normal_token_probability.toFixed(0) }}%</span>
              </label>
              <input
                v-model.number="config.normal_token_probability"
                type="range"
                min="0"
                max="100"
                step="1"
                class="range-slider w-full"
              />
              <div class="mt-1 flex justify-between text-[10px] text-gray-400">
                <span>0%</span><span>50%</span><span>100%</span>
              </div>
            </div>
            <!-- Min Tokens Threshold -->
            <div class="col-span-2 pt-1">
              <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                {{ t('admin.tokenControl.minTokens') }}
              </label>
              <input
                v-model.number="config.normal_token_min_tokens"
                type="number"
                min="0"
                step="100"
                class="w-full rounded-lg border border-gray-300 bg-white px-3 py-1.5 text-sm text-gray-900 focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500 dark:border-dark-600 dark:bg-dark-700 dark:text-white"
              />
              <p class="mt-1 text-[11px] text-gray-400 dark:text-gray-500">{{ t('admin.tokenControl.minTokensHint') }}</p>
            </div>
          </div>
          <!-- Preview -->
          <div class="rounded-b-xl bg-gray-50 px-5 py-3 dark:bg-dark-750">
            <p class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.tokenControl.previewNormal', {
                target: normalTargetText,
                range: config.normal_token_range.toFixed(1),
                prob: config.normal_token_probability.toFixed(0),
                min: config.normal_token_min_tokens,
              }) }}
            </p>
          </div>
        </div>

        <!-- Cache token card -->
        <div class="rounded-xl border border-gray-200 bg-white shadow-sm dark:border-dark-600 dark:bg-dark-800">
          <div class="border-b border-gray-100 px-5 py-3.5 dark:border-dark-700">
            <h2 class="text-sm font-semibold text-gray-800 dark:text-gray-100">
              {{ t('admin.tokenControl.cacheToken') }}
            </h2>
            <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.tokenControl.cacheTokenHint') }}</p>
          </div>
          <div class="grid grid-cols-2 gap-x-6 gap-y-4 px-5 py-4">
            <!-- Mode selection -->
            <div class="col-span-2 pb-1">
              <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                {{ t('admin.tokenControl.mode') }}
              </label>
              <select
                v-model="config.cache_token_mode"
                class="w-full rounded-lg border border-gray-300 bg-white px-3 py-1.5 text-sm text-gray-900 focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500 dark:border-dark-600 dark:bg-dark-700 dark:text-white"
              >
                <option value="all">{{ t('admin.tokenControl.modeAll') }}</option>
                <option value="read_only">{{ t('admin.tokenControl.modeReadOnly') }}</option>
                <option value="creation_only">{{ t('admin.tokenControl.modeCreationOnly') }}</option>
              </select>
            </div>
            <!-- Range -->
            <div>
              <label class="mb-1.5 block text-xs font-medium text-gray-600 dark:text-gray-400">
                {{ t('admin.tokenControl.range') }}
                <span class="ml-1 font-semibold text-gray-900 dark:text-white">{{ config.cache_token_range.toFixed(1) }}%</span>
              </label>
              <input
                v-model.number="config.cache_token_range"
                type="range"
                min="0"
                max="50"
                step="0.5"
                class="range-slider w-full"
              />
              <div class="mt-1 flex justify-between text-[10px] text-gray-400">
                <span>0%</span><span>25%</span><span>50%</span>
              </div>
            </div>
            <!-- Probability -->
            <div>
              <label class="mb-1.5 block text-xs font-medium text-gray-600 dark:text-gray-400">
                {{ t('admin.tokenControl.probability') }}
                <span class="ml-1 font-semibold text-gray-900 dark:text-white">{{ config.cache_token_probability.toFixed(0) }}%</span>
              </label>
              <input
                v-model.number="config.cache_token_probability"
                type="range"
                min="0"
                max="100"
                step="1"
                class="range-slider w-full"
              />
              <div class="mt-1 flex justify-between text-[10px] text-gray-400">
                <span>0%</span><span>50%</span><span>100%</span>
              </div>
            </div>
            <!-- Min Tokens Threshold -->
            <div class="col-span-2 pt-1">
              <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                {{ t('admin.tokenControl.minTokens') }}
              </label>
              <input
                v-model.number="config.cache_token_min_tokens"
                type="number"
                min="0"
                step="100"
                class="w-full rounded-lg border border-gray-300 bg-white px-3 py-1.5 text-sm text-gray-900 focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500 dark:border-dark-600 dark:bg-dark-700 dark:text-white"
              />
              <p class="mt-1 text-[11px] text-gray-400 dark:text-gray-500">{{ t('admin.tokenControl.minTokensHint') }}</p>
            </div>
          </div>
          <!-- Preview -->
          <div class="rounded-b-xl bg-gray-50 px-5 py-3 dark:bg-dark-750">
            <p class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.tokenControl.previewCache', {
                target: cacheTargetText,
                range: config.cache_token_range.toFixed(1),
                prob: config.cache_token_probability.toFixed(0),
                min: config.cache_token_min_tokens,
              }) }}
            </p>
          </div>
        </div>

        <!-- Group Cache Ratio Card -->
        <div class="rounded-xl border border-gray-200 bg-white shadow-sm dark:border-dark-600 dark:bg-dark-800">
          <div class="border-b border-gray-100 px-5 py-3.5 dark:border-dark-700">
            <h2 class="text-sm font-semibold text-gray-800 dark:text-gray-100">
              {{ t('admin.tokenControl.groupCacheTitle') }}
            </h2>
            <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.tokenControl.groupCacheHint') }}</p>
          </div>
          <div class="px-5 py-4">
            <!-- Add group dropdown and button -->
            <div class="flex items-center gap-3">
              <select
                v-model="selectedGroupToAdd"
                class="flex-1 rounded-lg border border-gray-300 bg-white px-3 py-1.5 text-sm text-gray-900 focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500 dark:border-dark-600 dark:bg-dark-700 dark:text-white"
              >
                <option value="" disabled>{{ t('admin.tokenControl.selectGroupPlaceholder') }}</option>
                <option v-for="g in availableGroupsForSelect" :key="g.id" :value="g.id">
                  {{ g.name }} (ID: {{ g.id }})
                </option>
              </select>
              <button
                type="button"
                :disabled="!selectedGroupToAdd"
                class="inline-flex items-center rounded-lg bg-primary-600 px-3.5 py-1.5 text-xs font-medium text-white transition-colors hover:bg-primary-700 disabled:cursor-not-allowed disabled:opacity-50"
                @click="addGroupRatioRule"
              >
                + {{ t('admin.tokenControl.addGroup') }}
              </button>
            </div>

            <!-- List of configured group rules -->
            <div v-if="config.group_cache_ratios && config.group_cache_ratios.length > 0" class="mt-4 space-y-3">
              <div
                v-for="(item, idx) in config.group_cache_ratios"
                :key="item.group_id"
                class="rounded-lg border border-gray-100 bg-gray-50/80 p-3.5 dark:border-dark-700 dark:bg-dark-750"
              >
                <div class="flex items-center justify-between">
                  <div class="flex items-center gap-2">
                    <span class="inline-flex items-center rounded-md bg-primary-50 px-2 py-0.5 text-xs font-medium text-primary-700 dark:bg-primary-950/50 dark:text-primary-300">
                      {{ item.group_name }}
                    </span>
                    <span class="text-xs text-gray-400 dark:text-gray-500">ID: {{ item.group_id }}</span>
                  </div>
                  <button
                    type="button"
                    class="text-xs text-red-500 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300"
                    @click="removeGroupRatioRule(idx)"
                  >
                    {{ t('admin.tokenControl.removeGroup') }}
                  </button>
                </div>
                <div class="mt-3">
                  <div class="flex items-center justify-between text-xs">
                    <span class="font-medium text-gray-600 dark:text-gray-400">{{ t('admin.tokenControl.groupRatioLabel') }}</span>
                    <span class="font-semibold text-gray-900 dark:text-white">{{ item.ratio.toFixed(0) }}%</span>
                  </div>
                  <input
                    v-model.number="item.ratio"
                    type="range"
                    min="0"
                    max="100"
                    step="1"
                    class="range-slider mt-1.5 w-full"
                  />
                  <div class="mt-1 flex justify-between text-[10px] text-gray-400">
                    <span>0% (全转常规Input)</span><span>50%</span><span>100% (全保留缓存)</span>
                  </div>
                  <p class="mt-1.5 text-[11px] text-gray-500 dark:text-gray-400">
                    💡 例如 1000 缓存：其中 <strong>{{ (1000 * item.ratio / 100).toFixed(0) }} Token</strong> 保留缓存优惠，剩余 <strong>{{ (1000 - 1000 * item.ratio / 100).toFixed(0) }} Token</strong> 转移到常规 Input 按原价计费。
                  </p>
                </div>
              </div>
            </div>
            <div v-else class="mt-3 rounded-lg border border-dashed border-gray-200 py-4 text-center text-xs text-gray-400 dark:border-dark-600">
              {{ t('admin.tokenControl.noGroupSelected') }}
            </div>
          </div>
        </div>
      </div>

      <!-- Save button -->
      <div class="mt-6 flex justify-end">
        <button
          id="token-control-save-btn"
          type="button"
          :disabled="saving"
          class="inline-flex items-center gap-2 rounded-lg bg-primary-600 px-5 py-2.5 text-sm font-medium text-white shadow-sm transition-all hover:bg-primary-700 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-60"
          @click="saveConfig"
        >
          <svg
            v-if="saving"
            class="h-4 w-4 animate-spin"
            fill="none"
            viewBox="0 0 24 24"
          >
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
          </svg>
          {{ saving ? t('common.saving') : t('common.save') }}
        </button>
      </div>
    </template>
  </div>
</template>

<style scoped>
.range-slider {
  @apply h-1.5 w-full cursor-pointer appearance-none rounded-full bg-gray-200 accent-primary-600 dark:bg-dark-600;
}
.range-slider::-webkit-slider-thumb {
  @apply h-4 w-4 cursor-pointer appearance-none rounded-full bg-primary-600 shadow-sm;
}
</style>
