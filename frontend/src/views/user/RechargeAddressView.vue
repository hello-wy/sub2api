<template>
  <AppLayout>
    <div class="flex h-[calc(100vh-8rem)] min-h-[620px] flex-col gap-4">
      <div class="card flex items-center justify-between gap-4 p-5">
        <div>
          <p class="text-xs font-semibold uppercase tracking-[0.24em] text-primary-500">
            {{ t('rechargeAddress.eyebrow', 'LDXP Shop') }}
          </p>
          <h1 class="mt-1 text-2xl font-bold text-gray-950 dark:text-white">
            {{ t('rechargeAddress.title') }}
          </h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-300">
            {{ t('rechargeAddress.description') }}
          </p>
        </div>
        <a :href="embeddedUrl" target="_blank" rel="noopener noreferrer" class="btn btn-secondary">
          <Icon name="externalLink" size="sm" class="mr-1.5" />
          {{ t('rechargeAddress.openInNewTab') }}
        </a>
      </div>

      <div class="card relative min-h-0 flex-1 overflow-hidden bg-gray-950">
        <div v-if="loading" class="absolute inset-0 z-10 flex items-center justify-center bg-white/80 dark:bg-dark-900/80">
          <div class="h-9 w-9 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
        </div>
        <div v-if="failed" class="absolute inset-x-4 top-4 z-20 rounded-xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800 shadow-lg dark:border-amber-900/50 dark:bg-amber-950/80 dark:text-amber-200">
          <div class="font-medium">{{ t('rechargeAddress.embedFailedTitle') }}</div>
          <p class="mt-1">{{ t('rechargeAddress.embedFailedDesc') }}</p>
        </div>
        <iframe
          :src="embeddedUrl"
          class="h-full w-full border-0 bg-white"
          allowfullscreen
          @load="loading = false"
          @error="markFailed"
        ></iframe>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { buildEmbeddedUrl, detectTheme } from '@/utils/embedded-url'

const RECHARGE_URL = 'https://pay.ldxp.cn/shop/68IO3EQ6'

const { t, locale } = useI18n()
const authStore = useAuthStore()
const loading = ref(true)
const failed = ref(false)
const pageTheme = ref<'light' | 'dark'>('light')
let themeObserver: MutationObserver | null = null

const embeddedUrl = computed(() =>
  buildEmbeddedUrl(RECHARGE_URL, authStore.user?.id, authStore.token, pageTheme.value, locale.value),
)

function markFailed() {
  loading.value = false
  failed.value = true
}

onMounted(() => {
  pageTheme.value = detectTheme()
  themeObserver = new MutationObserver(() => {
    pageTheme.value = detectTheme()
  })
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
})

onUnmounted(() => {
  themeObserver?.disconnect()
})
</script>
