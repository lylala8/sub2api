<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { getTokenJitterConfig, updateTokenJitterConfig, type TokenJitterConfig } from '@/api/tokenJitter'
import { useAppStore } from '@/stores'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const saving = ref(false)

const config = ref<TokenJitterConfig>({
  enabled: false,
  normal_token_range: 0,
  normal_token_probability: 0,
  cache_token_range: 0,
  cache_token_probability: 0,
})

async function fetchConfig() {
  loading.value = true
  try {
    config.value = await getTokenJitterConfig()
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
                max="10"
                step="0.5"
                class="range-slider w-full"
              />
              <div class="mt-1 flex justify-between text-[10px] text-gray-400">
                <span>0%</span><span>5%</span><span>10%</span>
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
          </div>
          <!-- Preview -->
          <div class="rounded-b-xl bg-gray-50 px-5 py-3 dark:bg-dark-750">
            <p class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.tokenControl.previewNormal', {
                range: config.normal_token_range.toFixed(1),
                prob: config.normal_token_probability.toFixed(0),
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
                max="10"
                step="0.5"
                class="range-slider w-full"
              />
              <div class="mt-1 flex justify-between text-[10px] text-gray-400">
                <span>0%</span><span>5%</span><span>10%</span>
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
          </div>
          <!-- Preview -->
          <div class="rounded-b-xl bg-gray-50 px-5 py-3 dark:bg-dark-750">
            <p class="text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.tokenControl.previewCache', {
                range: config.cache_token_range.toFixed(1),
                prob: config.cache_token_probability.toFixed(0),
              }) }}
            </p>
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
