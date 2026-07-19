<template>
  <div v-if="isMinimalVariant" class="auth-minimal-shell">
    <main class="auth-minimal-card" aria-label="账户登录">
      <slot />
      <div class="auth-minimal-footer">
        <slot name="footer" />
      </div>
    </main>
  </div>

  <div v-else class="relative flex min-h-screen items-center justify-center overflow-hidden p-4">
    <div
      class="absolute inset-0 bg-gradient-to-br from-gray-50 via-primary-50/30 to-gray-100 dark:from-dark-950 dark:via-dark-900 dark:to-dark-950"
    ></div>

    <div class="pointer-events-none absolute inset-0 overflow-hidden">
      <div
        class="absolute -right-40 -top-40 h-80 w-80 rounded-full bg-primary-400/20 blur-3xl"
      ></div>
      <div
        class="absolute -bottom-40 -left-40 h-80 w-80 rounded-full bg-primary-500/15 blur-3xl"
      ></div>
      <div
        class="absolute left-1/2 top-1/2 h-96 w-96 -translate-x-1/2 -translate-y-1/2 rounded-full bg-primary-300/10 blur-3xl"
      ></div>
      <div
        class="absolute inset-0 bg-[linear-gradient(rgba(22,119,255,0.03)_1px,transparent_1px),linear-gradient(90deg,rgba(22,119,255,0.03)_1px,transparent_1px)] bg-[size:64px_64px]"
      ></div>
    </div>

    <div class="relative z-10 w-full max-w-md">
      <div class="mb-8 text-center">
        <template v-if="settingsLoaded">
          <div
            class="mb-4 inline-flex h-16 w-16 items-center justify-center overflow-hidden rounded-2xl shadow-lg shadow-primary-500/30"
          >
            <img :src="siteLogo || '/logo.png'" alt="Logo" class="h-full w-full object-contain" />
          </div>
          <h1 class="text-gradient mb-2 text-3xl font-bold">
            {{ siteName }}
          </h1>
          <p class="text-sm text-gray-500 dark:text-dark-400">
            {{ siteSubtitle }}
          </p>
        </template>
      </div>

      <div class="card-glass rounded-2xl p-8 shadow-glass">
        <slot />
      </div>

      <div class="mt-6 text-center text-sm">
        <slot name="footer" />
      </div>

      <div class="mt-8 text-center text-xs text-gray-400 dark:text-dark-500">
        &copy; {{ currentYear }} {{ siteName }}. All rights reserved.
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'

const props = withDefaults(defineProps<{
  variant?: 'default' | 'minimal'
}>(), {
  variant: 'default'
})

const appStore = useAppStore()

const isMinimalVariant = computed(() => props.variant === 'minimal')
const siteName = computed(() => appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'Subscription to API Conversion Platform')
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)
const currentYear = computed(() => new Date().getFullYear())

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>

<style scoped>
.text-gradient {
  @apply bg-gradient-to-r from-primary-600 to-primary-500 bg-clip-text text-transparent;
}

.auth-minimal-shell {
  display: flex;
  min-height: 100vh;
  min-height: 100dvh;
  align-items: center;
  justify-content: center;
  padding: 24px 16px;
  background: #ffffff;
  color: #111827;
}

.auth-minimal-card {
  width: min(100%, 420px);
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #ffffff;
  padding: 30px;
  box-shadow: 0 18px 48px rgba(15, 23, 42, 0.08);
}

.auth-minimal-footer {
  margin-top: 18px;
  text-align: center;
  font-size: 13px;
}

.dark .auth-minimal-shell {
  background: #070b12;
  color: #f8fafc;
}

.dark .auth-minimal-card {
  border-color: #263548;
  background: #0b1422;
  box-shadow: 0 22px 56px rgba(0, 0, 0, 0.34);
}

@media (max-width: 480px) {
  .auth-minimal-shell {
    align-items: flex-start;
    padding-top: 72px;
  }

  .auth-minimal-card {
    padding: 24px 20px;
  }
}
</style>
