<template>
  <!-- Custom Home Content: Full Page Mode -->
  <div v-if="homeContent" class="min-h-screen">
    <!-- iframe mode -->
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <!-- HTML mode - SECURITY: homeContent is admin-only setting, XSS risk is acceptable -->
    <div v-else v-html="homeContent"></div>
  </div>

  <!-- Default Home Page -->
  <div
    v-else
    class="min-h-screen bg-[#f4f8ff] tracking-[-0.02em] text-[#06111f] dark:bg-[#050b14] dark:text-white"
    :style="{ fontFamily: `'Inter', sans-serif` }"
  >
    <nav class="fixed left-0 right-0 top-0 z-[100] flex h-[76px] items-center justify-between px-4 sm:px-6 lg:px-8">
      <router-link to="/home" class="flex items-center gap-3" aria-label="Solid API home">
        <span class="flex h-10 w-10 items-center justify-center">
          <img :src="siteLogo || '/logo.png'" alt="Solid API" class="h-8 w-8 object-contain" />
        </span>
        <span class="text-[21px] font-semibold tracking-[-0.04em] text-[#06111f] dark:text-white">Solid API</span>
      </router-link>

      <div
        class="absolute left-1/2 hidden -translate-x-1/2 items-center gap-1 rounded-full border border-[#d8e2ee] bg-white px-2 py-2 dark:border-[#263548] dark:bg-[#0d1724] md:flex"
      >
        <a class="rounded-full bg-[#06111f] px-4 py-1.5 text-sm font-semibold text-white dark:bg-white dark:text-[#06111f]" href="#home-hero">
          首页
        </a>
        <template v-for="item in navItems" :key="item.label">
          <router-link
            v-if="item.internal"
            class="inline-flex items-center gap-1.5 rounded-full px-4 py-1.5 text-sm font-semibold text-[#26374d] transition-colors hover:bg-[#1677ff]/10 hover:text-[#06111f] dark:text-white dark:hover:bg-white/12 dark:hover:text-white"
            :to="item.href"
          >
            <Icon :name="item.icon" size="xs" :stroke-width="2" />
            {{ item.label }}
          </router-link>
          <a
            v-else
            class="inline-flex items-center gap-1.5 rounded-full px-4 py-1.5 text-sm font-semibold text-[#26374d] transition-colors hover:bg-[#1677ff]/10 hover:text-[#06111f] dark:text-white dark:hover:bg-white/12 dark:hover:text-white"
            :href="item.href"
            :target="item.external ? '_blank' : undefined"
            :rel="item.external ? 'noopener noreferrer' : undefined"
          >
            <Icon :name="item.icon" size="xs" :stroke-width="2" />
            {{ item.label }}
          </a>
        </template>
      </div>

      <div class="flex items-center gap-2">
        <button
          class="inline-flex h-11 w-11 items-center justify-center rounded-full border border-[#d8e2ee] bg-white text-[#06111f] transition-colors hover:border-[#b9c9da] hover:bg-[#f4f8ff] active:scale-95 dark:border-[#263548] dark:bg-[#0d1724] dark:text-white dark:hover:border-[#3b526b] dark:hover:bg-[#121f2f]"
          type="button"
          :aria-label="isDark ? '切换到浅色模式' : '切换到深色模式'"
          :title="isDark ? '切换到浅色模式' : '切换到深色模式'"
          @click="toggleTheme"
        >
          <Icon :name="isDark ? 'sun' : 'moon'" size="md" :stroke-width="2" />
        </button>

        <router-link
          v-if="isAuthenticated"
          :to="dashboardPath"
          class="hidden items-center gap-2 rounded-full bg-[#06111f] px-5 py-2.5 text-sm font-semibold text-white shadow-[0_18px_45px_rgba(6,17,31,0.18)] transition-all hover:-translate-y-0.5 hover:bg-[#102235] dark:bg-white dark:text-[#06111f] dark:shadow-[0_18px_45px_rgba(255,255,255,0.12)] dark:hover:bg-[#eef6ff] md:inline-flex"
        >
          进入控制台
          <Icon name="arrowRight" size="sm" :stroke-width="2" />
        </router-link>

        <button
          v-else
          type="button"
          class="hidden items-center gap-2 rounded-full bg-[#06111f] px-5 py-2.5 text-sm font-semibold text-white shadow-[0_18px_45px_rgba(6,17,31,0.18)] transition-all hover:-translate-y-0.5 hover:bg-[#102235] dark:bg-white dark:text-[#06111f] dark:shadow-[0_18px_45px_rgba(255,255,255,0.12)] dark:hover:bg-[#eef6ff] md:inline-flex"
          @click="openInlineLogin"
        >
          开始使用
          <Icon name="arrowRight" size="sm" :stroke-width="2" />
        </button>

        <button
          class="flex h-11 w-11 items-center justify-center rounded-full border border-[#d8e2ee] bg-white text-[#06111f] transition-colors hover:border-[#b9c9da] hover:bg-[#f4f8ff] md:hidden dark:border-[#263548] dark:bg-[#0d1724] dark:text-white dark:hover:border-[#3b526b] dark:hover:bg-[#121f2f]"
          type="button"
          aria-label="打开菜单"
        >
          <Icon name="menu" size="md" :stroke-width="2" />
        </button>
      </div>
    </nav>

    <section
      id="home-hero"
      class="relative min-h-[100dvh] w-full overflow-x-hidden bg-[#f4f8ff] dark:bg-[#050b14]"
      @mouseleave="hideSpotlight"
    >
      <img
        :key="`base-${currentHeroImage}`"
        :src="currentHeroImage"
        alt=""
        class="hero-zoom pointer-events-none absolute inset-0 z-10 h-full w-full object-cover object-center"
        :style="{ filter: baseImageFilter }"
        loading="eager"
        decoding="async"
        fetchpriority="high"
        aria-hidden="true"
      />

      <img
        :key="`reveal-${currentHeroImage}`"
        :src="currentHeroImage"
        alt=""
        class="hero-zoom pointer-events-none absolute inset-0 z-30 h-full w-full object-cover object-center"
        :style="{
          filter: revealImageFilter,
          maskImage: spotlightMask,
          WebkitMaskImage: spotlightMask
        }"
        decoding="async"
        aria-hidden="true"
      />

      <div
        class="pointer-events-none absolute inset-0 z-[31]"
        :style="{ background: spotlightGlow }"
        aria-hidden="true"
      />

      <div class="pointer-events-none absolute inset-0 z-40" :style="{ background: primaryOverlay }"></div>
      <div class="pointer-events-none absolute inset-0 z-40" :style="{ background: verticalOverlay }"></div>

      <div class="absolute left-0 top-[23%] z-50 w-full px-7 sm:top-[20%] sm:px-10 lg:top-1/2 lg:-translate-y-[46%] lg:pr-14 lg:pl-[clamp(4rem,6vw,7rem)]">
        <Transition name="home-login" mode="out-in">
          <div v-if="!showInlineLogin" key="hero-copy" class="max-w-[840px]">
          <p
            class="hero-anim hero-fade mb-6 text-sm font-semibold tracking-[0.08em] text-[#1677ff] dark:text-[#a6d3ff]"
            :style="{ animationDelay: '0.12s' }"
          >
            AI API Gateway · 企业级中转
          </p>

          <h1 class="relative max-w-[720px] text-[64px] font-semibold leading-[0.86] tracking-[-0.08em] [text-shadow:0_2px_24px_rgba(255,255,255,0.55)] sm:text-[96px] md:text-[124px] lg:text-[150px] dark:[text-shadow:0_2px_30px_rgba(0,0,0,0.58)]">
            <span
              class="solidapi-wordmark block"
            >
              SolidAPI
            </span>
          </h1>

          <p
            class="hero-anim hero-fade mt-7 max-w-[560px] text-[15px] font-semibold leading-7 text-[#22344a] sm:text-base dark:text-white"
            :style="{ animationDelay: '0.58s' }"
          >
            一站式大模型 API 聚合与中转平台
          </p>

          <div
            class="hero-anim hero-fade mt-8 flex flex-wrap items-center gap-3"
            :style="{ animationDelay: '0.74s' }"
          >
            <router-link
              v-if="isAuthenticated"
              :to="dashboardPath"
              class="inline-flex items-center gap-2 rounded-full bg-[#1677ff] px-7 py-3 text-sm font-semibold text-white shadow-[0_18px_42px_rgba(22,119,255,0.32)] transition-all hover:scale-[1.03] hover:bg-[#2488ff] active:scale-95 dark:bg-[#2488ff] dark:shadow-[0_18px_42px_rgba(36,136,255,0.36)]"
            >
              开始使用
              <Icon name="arrowRight" size="sm" :stroke-width="2" />
            </router-link>
            <button
              v-else
              type="button"
              class="inline-flex items-center gap-2 rounded-full bg-[#1677ff] px-7 py-3 text-sm font-semibold text-white shadow-[0_18px_42px_rgba(22,119,255,0.32)] transition-all hover:scale-[1.03] hover:bg-[#2488ff] active:scale-95 dark:bg-[#2488ff] dark:shadow-[0_18px_42px_rgba(36,136,255,0.36)]"
              @click="openInlineLogin"
            >
              开始使用
              <Icon name="arrowRight" size="sm" :stroke-width="2" />
            </button>
            <router-link
              to="/models"
              class="inline-flex items-center gap-2 rounded-full border border-[#1677ff]/18 bg-white/76 px-7 py-3 text-sm font-semibold text-[#06111f] shadow-[0_16px_36px_rgba(20,68,140,0.10)] backdrop-blur-xl transition-all hover:border-[#1677ff]/36 hover:bg-white active:scale-95 dark:border-white/18 dark:bg-white/14 dark:text-white dark:shadow-none dark:hover:border-[#79b8ff]/55 dark:hover:bg-white/18"
            >
              查看价格
              <Icon name="creditCard" size="sm" :stroke-width="1.8" />
            </router-link>
          </div>

          <div
            class="hero-anim hero-fade mt-10 max-w-[620px]"
            :style="{ animationDelay: '0.9s' }"
          >
            <LogoLoop
              :logos="providerLogos"
              :speed="42"
              direction="right"
              :logo-height="28"
              :gap="32"
              :hover-speed="0"
              :scale-on-hover="true"
              :fade-out="true"
              :fade-out-color="isDark ? '#050b14' : '#f4f8ff'"
              aria-label="Supported LLM providers"
            >
              <template #renderItem="{ item }">
                <div class="group/item inline-flex items-center justify-center transition-transform duration-300 group-hover/item:scale-110">
                  <ProviderLogo :provider="item.title || ''" />
                  <span class="sr-only">{{ item.title }}</span>
                </div>
              </template>
            </LogoLoop>
          </div>
        </div>

          <div v-else key="inline-login" class="home-inline-login">
            <div class="home-inline-card">
              <button
                type="button"
                class="home-inline-close"
                aria-label="关闭登录面板"
                title="关闭登录面板"
                @click="closeInlineLogin"
              >
                <Icon name="x" size="sm" :stroke-width="2" />
              </button>
              <div class="home-inline-kicker">
                <span class="home-inline-kicker-dot"></span>
                安全访问 · AI API Gateway
              </div>
              <LoginView :embedded="true" />
            </div>
          </div>
        </Transition>
      </div>

    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useAuthStore, useAppStore } from '@/stores'
import LogoLoop, { type LogoItem } from '@/components/home/LogoLoop.vue'
import ProviderLogo from '@/components/home/ProviderLogo.vue'
import Icon from '@/components/icons/Icon.vue'
import LoginView from '@/views/auth/LoginView.vue'
import { sanitizeUrl } from '@/utils/url'

type IconName = InstanceType<typeof Icon>['$props']['name']

const HERO_IMAGE_DARK = '/home/solid-api-blue-core.webp'
const HERO_IMAGE_LIGHT = '/home/solid-api-blue-core-light.webp'
const SPOTLIGHT_R = 260
const authStore = useAuthStore()
const appStore = useAppStore()

const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', {
  allowRelative: true,
  allowDataUrl: true
}))
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')

const navItems = computed<Array<{ label: string, href: string, icon: IconName, internal?: boolean, external?: boolean }>>(() => [
  { label: 'API 接入', href: '/available-channels', icon: 'link', internal: true },
  { label: '模型价格', href: '/models', icon: 'creditCard', internal: true },
  { label: '运行状态', href: '/monitor', icon: 'chart', internal: true },
  { label: '文档', href: docUrl.value || '/docs', icon: 'book', internal: !docUrl.value, external: Boolean(docUrl.value) }
])

const providerLogos: LogoItem[] = [
  { node: 'OpenAI', title: 'OpenAI' },
  { node: 'Anthropic', title: 'Anthropic' },
  { node: 'Gemini', title: 'Gemini' },
  { node: 'DeepSeek', title: 'DeepSeek' },
  { node: 'Grok', title: 'Grok' },
  { node: 'MiniMax', title: 'MiniMax' },
  { node: 'Claude', title: 'Claude' },
  { node: 'Kimi', title: 'Kimi' },
  { node: 'Qwen', title: 'Qwen' }
]

const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

const isDark = ref(document.documentElement.classList.contains('dark'))
const showInlineLogin = ref(false)
const cursorPos = ref({ x: -999, y: -999 })
const mouse = { x: -999, y: -999 }
const smooth = { x: -999, y: -999 }
const rafRef = ref<number | null>(null)
let themeObserver: MutationObserver | null = null

const currentHeroImage = computed(() => isDark.value ? HERO_IMAGE_DARK : HERO_IMAGE_LIGHT)
const baseImageFilter = computed(() => (
  isDark.value
    ? 'brightness(0.88) saturate(1.08) contrast(1.08)'
    : 'brightness(1.03) saturate(1.02) contrast(1.02)'
))
const revealImageFilter = computed(() => (
  isDark.value
    ? 'brightness(1.16) saturate(1.16) contrast(1.08)'
    : 'brightness(1.04) saturate(1.08) contrast(1.04)'
))
const spotlightMask = computed(() => {
  const { x, y } = cursorPos.value
  return `radial-gradient(circle ${SPOTLIGHT_R}px at ${x}px ${y}px, black 0%, black 40%, rgba(0, 0, 0, 0.75) 60%, rgba(0, 0, 0, 0.4) 75%, rgba(0, 0, 0, 0.12) 88%, transparent 100%)`
})
const spotlightGlow = computed(() => {
  const { x, y } = cursorPos.value
  return isDark.value
    ? `radial-gradient(circle ${SPOTLIGHT_R}px at ${x}px ${y}px, rgba(102, 185, 255, 0.10) 0%, rgba(40, 132, 255, 0.05) 48%, transparent 100%)`
    : `radial-gradient(circle ${SPOTLIGHT_R}px at ${x}px ${y}px, rgba(22, 119, 255, 0.18) 0%, rgba(22, 119, 255, 0.10) 48%, rgba(22, 119, 255, 0.03) 74%, transparent 100%)`
})
const primaryOverlay = computed(() => (
  isDark.value
    ? 'radial-gradient(circle at 72% 38%, rgba(0,119,255,0.09), transparent 34%), linear-gradient(90deg, rgba(5,11,20,0.72) 0%, rgba(5,11,20,0.54) 34%, rgba(5,11,20,0.08) 68%, rgba(5,11,20,0.18) 100%)'
    : 'radial-gradient(circle at 72% 38%, rgba(22,119,255,0.05), transparent 34%), linear-gradient(90deg, rgba(245,249,255,0.76) 0%, rgba(245,249,255,0.56) 34%, rgba(245,249,255,0.08) 68%, rgba(245,249,255,0.16) 100%)'
))
const verticalOverlay = computed(() => (
  isDark.value
    ? 'linear-gradient(180deg, rgba(5,11,20,0.08) 0%, transparent 46%, rgba(0,0,0,0.28) 100%)'
    : 'linear-gradient(180deg, rgba(255,255,255,0.22) 0%, transparent 46%, rgba(235,243,255,0.22) 100%)'
))

const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')

function openInlineLogin() {
  if (isAuthenticated.value) return
  showInlineLogin.value = true
}

function closeInlineLogin() {
  showInlineLogin.value = false
}

function handleMouseMove(event: MouseEvent) {
  mouse.x = event.clientX
  mouse.y = event.clientY
}

function animateSpotlight() {
  smooth.x += (mouse.x - smooth.x) * 0.1
  smooth.y += (mouse.y - smooth.y) * 0.1
  cursorPos.value = { x: smooth.x, y: smooth.y }
  rafRef.value = requestAnimationFrame(animateSpotlight)
}

function hideSpotlight() {
  mouse.x = -999
  mouse.y = -999
}

function toggleTheme() {
  const nextIsDark = !isDark.value
  document.documentElement.classList.toggle('dark', nextIsDark)
  localStorage.setItem('theme', nextIsDark ? 'dark' : 'light')
  isDark.value = nextIsDark
}

function updateThemeState() {
  isDark.value = document.documentElement.classList.contains('dark')
}

function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  const shouldUseDark =
    savedTheme === 'dark' ||
    (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)

  document.documentElement.classList.toggle('dark', shouldUseDark)
  updateThemeState()

  if (themeObserver) {
    themeObserver.disconnect()
  }

  themeObserver = new MutationObserver(updateThemeState)
  themeObserver.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ['class']
  })
}

onMounted(() => {
  initTheme()
  authStore.checkAuth()

  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }

  window.addEventListener('mousemove', handleMouseMove, { passive: true })
  rafRef.value = requestAnimationFrame(animateSpotlight)
})

onBeforeUnmount(() => {
  window.removeEventListener('mousemove', handleMouseMove)
  themeObserver?.disconnect()
  if (rafRef.value !== null) {
    cancelAnimationFrame(rafRef.value)
  }
})
</script>

<style scoped>
.home-inline-login {
  width: min(100%, 448px);
}

.home-inline-card {
  position: relative;
  border: 1px solid rgba(148, 163, 184, 0.34);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.96);
  padding: 30px;
  box-shadow: 0 28px 72px rgba(30, 64, 175, 0.18), 0 2px 10px rgba(15, 23, 42, 0.06);
}

.home-inline-close {
  position: absolute;
  top: 16px;
  right: 16px;
  display: inline-flex;
  width: 32px;
  height: 32px;
  align-items: center;
  justify-content: center;
  border: 1px solid #dbe3ee;
  border-radius: 7px;
  color: #64748b;
  transition: border-color 180ms ease, background-color 180ms ease, color 180ms ease;
}

.home-inline-close:hover {
  border-color: #a8b8cc;
  background: #f8fafc;
  color: #1677ff;
}

.home-inline-kicker {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 22px;
  color: #64748b;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.home-inline-kicker-dot {
  width: 7px;
  height: 7px;
  flex: 0 0 auto;
  border-radius: 999px;
  background: #1677ff;
  box-shadow: 0 0 0 4px rgba(22, 119, 255, 0.12);
}

.home-login-enter-active,
.home-login-leave-active {
  transition: opacity 260ms ease, transform 260ms ease;
}

.home-login-enter-from {
  opacity: 0;
  transform: translateX(-28px) scale(0.98);
}

.home-login-leave-to {
  opacity: 0;
  transform: translateX(-18px) scale(0.98);
}

.dark .home-inline-card {
  border-color: rgba(71, 85, 105, 0.7);
  background: rgba(11, 20, 34, 0.98);
  box-shadow: 0 30px 82px rgba(0, 0, 0, 0.46), 0 0 0 1px rgba(96, 165, 250, 0.04);
}

.dark .home-inline-close {
  border-color: #334155;
  color: #94a3b8;
}

.dark .home-inline-close:hover {
  border-color: #52647b;
  background: #101c2d;
  color: #60a5fa;
}

.dark .home-inline-kicker {
  color: #94a3b8;
}

@media (max-width: 767px) {
  .home-inline-login {
    width: 100%;
  }

  .home-inline-card {
    padding: 24px 20px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .home-login-enter-active,
  .home-login-leave-active,
  .home-inline-close {
    transition: none;
  }
}
</style>
