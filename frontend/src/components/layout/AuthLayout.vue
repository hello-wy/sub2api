<template>
  <div v-if="isHomeVariant" class="auth-home-shell">
    <img
      :src="homeHeroImage"
      alt=""
      aria-hidden="true"
      class="auth-home-backdrop"
    />
    <div class="auth-home-overlay" aria-hidden="true"></div>

    <header class="auth-home-header">
      <router-link to="/home" class="auth-home-brand" :aria-label="`${siteName} 首页`">
        <img :src="siteLogo || '/logo.png'" alt="" class="auth-home-logo" />
        <span>{{ siteName }}</span>
      </router-link>

      <div class="auth-home-actions">
        <router-link to="/home" class="auth-home-link">
          <Icon name="arrowLeft" size="sm" />
          <span>{{ t('auth.backToHome') }}</span>
        </router-link>
        <button
          type="button"
          class="auth-home-theme"
          :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
          :aria-label="isDark ? t('home.switchToLight') : t('home.switchToDark')"
          @click="toggleTheme"
        >
          <Icon :name="isDark ? 'sun' : 'moon'" size="md" />
        </button>
      </div>
    </header>

    <main class="auth-home-main">
      <section class="auth-home-panel" aria-label="账户登录">
        <div class="auth-home-card">
          <div class="auth-home-kicker">
            <span class="auth-home-kicker-dot"></span>
            {{ t('auth.secureAccess') }} · AI API Gateway
          </div>
          <slot />
        </div>

        <div class="auth-home-footer">
          <slot name="footer" />
        </div>

        <p class="auth-home-copyright">
          &copy; {{ currentYear }} {{ siteName }}. All rights reserved.
        </p>
      </section>
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
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'

const props = withDefaults(defineProps<{
  variant?: 'default' | 'home'
}>(), {
  variant: 'default'
})

const appStore = useAppStore()
const { t } = useI18n()
const isDark = ref(false)
let themeObserver: MutationObserver | null = null

const isHomeVariant = computed(() => props.variant === 'home')
const siteName = computed(() => appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'Subscription to API Conversion Platform')
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)
const currentYear = computed(() => new Date().getFullYear())
const homeHeroImage = computed(() => (
  isDark.value
    ? '/home/solid-api-blue-core.webp'
    : '/home/solid-api-blue-core-light.webp'
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

  if (!isHomeVariant.value) return

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

.auth-home-shell {
  position: relative;
  min-height: 100vh;
  min-height: 100dvh;
  overflow-x: hidden;
  background: #edf4ff;
  color: #111827;
}

.auth-home-backdrop,
.auth-home-overlay {
  position: fixed;
  inset: 0;
  width: 100%;
  height: 100%;
}

.auth-home-backdrop {
  object-fit: cover;
  object-position: center;
}

.auth-home-overlay {
  background: linear-gradient(90deg, rgba(239, 246, 255, 0.94) 0%, rgba(239, 246, 255, 0.72) 36%, rgba(239, 246, 255, 0.05) 70%);
}

.auth-home-header {
  position: relative;
  z-index: 2;
  display: flex;
  min-height: 72px;
  align-items: center;
  justify-content: space-between;
  padding: 14px 32px;
}

.auth-home-brand,
.auth-home-actions,
.auth-home-link {
  display: flex;
  align-items: center;
}

.auth-home-brand {
  gap: 10px;
  color: #0f172a;
  font-size: 17px;
  font-weight: 700;
}

.auth-home-logo {
  width: 36px;
  height: 36px;
  border-radius: 7px;
  object-fit: contain;
}

.auth-home-actions {
  gap: 8px;
}

.auth-home-link,
.auth-home-theme {
  min-height: 38px;
  border: 1px solid rgba(148, 163, 184, 0.36);
  border-radius: 7px;
  background: rgba(255, 255, 255, 0.92);
  color: #334155;
  box-shadow: 0 4px 16px rgba(30, 64, 175, 0.06);
  transition: border-color 180ms ease, background-color 180ms ease, color 180ms ease;
}

.auth-home-link {
  gap: 7px;
  padding: 0 13px;
  font-size: 13px;
  font-weight: 600;
}

.auth-home-theme {
  display: inline-flex;
  width: 38px;
  align-items: center;
  justify-content: center;
}

.auth-home-link:hover,
.auth-home-theme:hover {
  border-color: rgba(22, 119, 255, 0.5);
  background: #ffffff;
  color: #1677ff;
}

.auth-home-main {
  position: relative;
  z-index: 1;
  display: flex;
  min-height: calc(100vh - 72px);
  min-height: calc(100dvh - 72px);
  align-items: center;
  padding: 24px 7vw 40px;
}

.auth-home-panel {
  width: min(100%, 448px);
  margin: auto 0;
}

.auth-home-card {
  border: 1px solid rgba(148, 163, 184, 0.28);
  border-radius: 8px;
  background: #ffffff;
  padding: 30px;
  box-shadow: 0 26px 70px rgba(30, 64, 175, 0.16), 0 2px 8px rgba(15, 23, 42, 0.06);
}

.auth-home-kicker {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 22px;
  color: #64748b;
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
}

.auth-home-kicker-dot {
  width: 7px;
  height: 7px;
  border-radius: 999px;
  background: #1677ff;
  box-shadow: 0 0 0 4px rgba(22, 119, 255, 0.12);
}

.auth-home-footer {
  margin-top: 18px;
  text-align: center;
  font-size: 13px;
}

.auth-home-copyright {
  margin-top: 18px;
  text-align: center;
  color: #64748b;
  font-size: 11px;
}

.dark .auth-home-shell {
  background: #050b14;
  color: #f8fafc;
}

.dark .auth-home-overlay {
  background: linear-gradient(90deg, rgba(5, 11, 20, 0.96) 0%, rgba(5, 11, 20, 0.78) 38%, rgba(5, 11, 20, 0.08) 72%);
}

.dark .auth-home-brand {
  color: #f8fafc;
}

.dark .auth-home-link,
.dark .auth-home-theme {
  border-color: rgba(100, 116, 139, 0.46);
  background: rgba(10, 20, 34, 0.92);
  color: #cbd5e1;
  box-shadow: none;
}

.dark .auth-home-link:hover,
.dark .auth-home-theme:hover {
  border-color: rgba(96, 165, 250, 0.62);
  background: #0d1a2b;
  color: #60a5fa;
}

.dark .auth-home-card {
  border-color: rgba(71, 85, 105, 0.66);
  background: #0b1422;
  box-shadow: 0 28px 80px rgba(0, 0, 0, 0.42), 0 0 0 1px rgba(96, 165, 250, 0.04);
}

.dark .auth-home-kicker,
.dark .auth-home-copyright {
  color: #94a3b8;
}

@media (max-width: 767px) {
  .auth-home-overlay {
    background: rgba(239, 246, 255, 0.86);
  }

  .auth-home-header {
    min-height: 64px;
    padding: 12px 16px;
  }

  .auth-home-brand span {
    display: none;
  }

  .auth-home-main {
    min-height: calc(100vh - 64px);
    min-height: calc(100dvh - 64px);
    justify-content: center;
    padding: 18px 16px 30px;
  }

  .auth-home-panel {
    margin: auto;
  }

  .auth-home-card {
    padding: 24px 20px;
  }

  .dark .auth-home-overlay {
    background: rgba(5, 11, 20, 0.88);
  }
}

@media (max-width: 380px) {
  .auth-home-link span {
    display: none;
  }

  .auth-home-link {
    width: 38px;
    justify-content: center;
    padding: 0;
  }
}

@media (prefers-reduced-motion: reduce) {
  .auth-home-link,
  .auth-home-theme {
    transition: none;
  }
}
</style>
