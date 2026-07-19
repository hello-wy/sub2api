<template>
  <div v-if="isMinimalVariant" class="auth-minimal-shell">
    <router-link to="/home" class="auth-corner-brand" :aria-label="`${siteName} 首页`">
      <img :src="brandLockup" :alt="siteName" class="auth-corner-lockup" />
    </router-link>

    <div class="auth-minimal-stack">
      <router-link to="/home" class="auth-minimal-brand" :aria-label="`${siteName} 首页`">
        <img :src="brandLockup" :alt="siteName" class="auth-minimal-lockup" />
      </router-link>

      <main class="auth-minimal-card" aria-label="账户登录">
        <div class="auth-minimal-tools">
          <LocaleSwitcher class="auth-minimal-locale" toolbar />
          <button
            type="button"
            class="auth-minimal-tool-button"
            :aria-label="isDark ? t('nav.lightMode') : t('nav.darkMode')"
            :title="isDark ? t('nav.lightMode') : t('nav.darkMode')"
            @click="toggleTheme"
          >
            <Icon :name="isDark ? 'sun' : 'moon'" size="md" />
          </button>
        </div>

        <slot />
        <div class="auth-minimal-footer">
          <slot name="footer" />
        </div>
      </main>
    </div>
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
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'

const props = withDefaults(defineProps<{
  variant?: 'default' | 'minimal'
}>(), {
  variant: 'default'
})

const appStore = useAppStore()
const { t } = useI18n()
const isDark = ref(document.documentElement.classList.contains('dark'))
let themeObserver: MutationObserver | null = null

const isMinimalVariant = computed(() => props.variant === 'minimal')
const siteName = computed(() => appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'Subscription to API Conversion Platform')
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)
const currentYear = computed(() => new Date().getFullYear())
const brandLockup = computed(() => (
  isDark.value
    ? '/brand/solidapi-lockup-dark.png'
    : '/brand/solidapi-lockup-light.png'
))

function updateThemeState() {
  isDark.value = document.documentElement.classList.contains('dark')
}

function toggleTheme() {
  const nextIsDark = !isDark.value
  document.documentElement.classList.toggle('dark', nextIsDark)
  localStorage.setItem('theme', nextIsDark ? 'dark' : 'light')
  isDark.value = nextIsDark
}

onMounted(() => {
  appStore.fetchPublicSettings()
  updateThemeState()
  themeObserver = new MutationObserver(updateThemeState)
  themeObserver.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ['class']
  })
})

onBeforeUnmount(() => {
  themeObserver?.disconnect()
})
</script>

<style scoped>
.text-gradient {
  @apply bg-gradient-to-r from-primary-600 to-primary-500 bg-clip-text text-transparent;
}

.auth-minimal-shell {
  position: relative;
  display: flex;
  min-height: 100vh;
  min-height: 100dvh;
  align-items: center;
  justify-content: center;
  padding: 24px 16px;
  background-color: #f0f7ff;
  color: #111827;
}

.auth-minimal-shell::before {
  position: fixed;
  inset: 0;
  background:
    linear-gradient(135deg, rgba(52, 124, 215, 0.12) 0%, rgba(52, 124, 215, 0) 43%),
    linear-gradient(315deg, rgba(73, 183, 216, 0.1) 0%, rgba(73, 183, 216, 0) 46%);
  content: '';
  pointer-events: none;
}

.auth-corner-brand {
  position: absolute;
  top: 24px;
  left: 28px;
  display: flex;
  align-items: center;
}

.auth-corner-lockup {
  width: 144px;
  height: auto;
  object-fit: contain;
}

.auth-minimal-stack {
  width: min(100%, 420px);
}

.auth-minimal-brand {
  display: flex;
  width: fit-content;
  align-items: center;
  margin: 0 auto 24px;
}

.auth-minimal-lockup {
  width: 178px;
  height: auto;
  object-fit: contain;
  filter: drop-shadow(0 8px 16px rgba(30, 64, 175, 0.16));
}

.auth-minimal-card {
  position: relative;
  width: 100%;
  border: 1px solid rgba(255, 255, 255, 0.72);
  border-radius: 20px;
  background: rgba(255, 255, 255, 0.5);
  padding: 34px;
  box-shadow:
    0 2px 7px rgba(39, 72, 112, 0.08),
    0 18px 42px rgba(39, 72, 112, 0.14),
    inset 0 1px 0 rgba(255, 255, 255, 0.8);
  backdrop-filter: blur(22px) saturate(1.2);
  -webkit-backdrop-filter: blur(22px) saturate(1.2);
}

.auth-minimal-tools {
  position: absolute;
  top: 18px;
  right: 18px;
  display: flex;
  align-items: center;
  gap: 2px;
  padding: 3px;
  border: 1px solid rgba(112, 145, 181, 0.2);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.34);
}

.auth-minimal-tools :deep(.header-tool-button),
.auth-minimal-tool-button {
  display: inline-flex;
  width: 32px;
  height: 32px;
  align-items: center;
  justify-content: center;
  border: 0;
  border-radius: 9px;
  background: transparent;
  color: #334155;
  transition: border-color 160ms ease, background-color 160ms ease, color 160ms ease;
}

.auth-minimal-tools :deep(.header-tool-button:hover:not(:disabled)),
.auth-minimal-tool-button:hover {
  background: rgba(255, 255, 255, 0.72);
  color: #1677ff;
}

.auth-minimal-footer {
  margin-top: 18px;
  text-align: center;
  font-size: 13px;
}

.dark .auth-minimal-shell {
  background-color: #0c1d31;
  color: #f8fafc;
}

.dark .auth-minimal-shell::before {
  background:
    linear-gradient(135deg, rgba(40, 103, 174, 0.2) 0%, rgba(40, 103, 174, 0) 44%),
    linear-gradient(315deg, rgba(24, 111, 139, 0.18) 0%, rgba(24, 111, 139, 0) 47%);
}

.dark .auth-minimal-tools {
  border-color: rgba(163, 207, 255, 0.12);
  background: rgba(17, 38, 62, 0.42);
}

.dark .auth-minimal-tools :deep(.header-tool-button),
.dark .auth-minimal-tool-button {
  background: transparent;
  color: #cbd5e1;
}

.dark .auth-minimal-tools :deep(.header-tool-button:hover:not(:disabled)),
.dark .auth-minimal-tool-button:hover {
  background: rgba(42, 75, 108, 0.72);
  color: #60a5fa;
}

.dark .auth-minimal-card {
  border-color: rgba(148, 191, 236, 0.2);
  background: rgba(8, 16, 28, 0.62);
  box-shadow:
    0 2px 7px rgba(0, 0, 0, 0.22),
    0 20px 46px rgba(0, 0, 0, 0.32),
    inset 0 1px 0 rgba(191, 219, 254, 0.11);
}

@media (max-width: 480px) {
  .auth-minimal-shell {
    align-items: flex-start;
    padding-top: 72px;
  }

  .auth-minimal-card {
    border-radius: 18px;
    padding: 30px 20px 26px;
  }

  .auth-minimal-brand {
    margin-bottom: 18px;
  }

  .auth-corner-brand {
    top: 18px;
    left: 18px;
  }

  .auth-corner-lockup {
    width: 120px;
    height: auto;
    object-fit: contain;
  }
}
</style>
