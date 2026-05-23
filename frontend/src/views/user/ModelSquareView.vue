<template>
  <AppLayout>
    <div class="model-square min-h-full rounded-[2rem] p-5 text-white md:p-8">
      <section class="rounded-[1.75rem] border border-white/10 bg-white/[0.08] p-6 shadow-2xl backdrop-blur-xl md:p-8">
        <p class="text-xs font-semibold uppercase tracking-[0.32em] text-cyan-200">{{ t('modelSquare.eyebrow') }}</p>
        <div class="mt-4 flex flex-col gap-6 lg:flex-row lg:items-end lg:justify-between">
          <div class="max-w-2xl">
            <h1 class="text-4xl font-black tracking-tight md:text-5xl">{{ t('modelSquare.title') }}</h1>
            <p class="mt-3 text-sm leading-6 text-slate-200 md:text-base">{{ t('modelSquare.description') }}</p>
          </div>
          <div class="grid grid-cols-3 gap-3">
            <div v-for="stat in stats" :key="stat.label" class="rounded-2xl border border-white/10 bg-black/20 p-4 text-center">
              <div class="text-2xl font-black">{{ stat.value }}</div>
              <div class="mt-1 text-[11px] uppercase tracking-widest text-slate-300">{{ stat.label }}</div>
            </div>
          </div>
        </div>
      </section>

      <section class="mt-5 rounded-[1.5rem] border border-white/10 bg-black/25 p-4 backdrop-blur-xl">
        <div class="grid gap-3 lg:grid-cols-[1fr_180px_180px_220px]">
          <div class="relative">
            <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
            <input v-model="searchQuery" class="model-input pl-10" :placeholder="t('modelSquare.searchPlaceholder')" />
          </div>
          <select v-model="platformFilter" class="model-input">
            <option value="">{{ t('modelSquare.filters.allPlatforms') }}</option>
            <option v-for="platform in platforms" :key="platform" :value="platform">{{ platform }}</option>
          </select>
          <select v-model="billingFilter" class="model-input">
            <option value="">{{ t('modelSquare.filters.allBillingModes') }}</option>
            <option value="token">{{ t('modelSquare.billing.token') }}</option>
            <option value="per_request">{{ t('modelSquare.billing.perRequest') }}</option>
            <option value="image">{{ t('modelSquare.billing.image') }}</option>
          </select>
          <select v-model="groupFilter" class="model-input">
            <option value="">{{ t('modelSquare.filters.allGroups') }}</option>
            <option v-for="group in groupOptions" :key="group.id" :value="String(group.id)">
              {{ group.name }} · {{ group.rate }}x
            </option>
          </select>
        </div>
      </section>

      <div v-if="loading" class="flex justify-center py-20">
        <div class="h-10 w-10 animate-spin rounded-full border-2 border-cyan-300 border-t-transparent"></div>
      </div>
      <div v-else-if="filteredModels.length === 0" class="py-20 text-center text-slate-300">
        {{ t('modelSquare.empty') }}
      </div>
      <section v-else class="mt-5 grid gap-4 xl:grid-cols-2 2xl:grid-cols-3">
        <article v-for="item in filteredModels" :key="item.key" class="model-card">
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <span class="rounded-full bg-white/10 px-2 py-1 text-[11px] font-semibold uppercase tracking-wider text-cyan-100">{{ item.platform }}</span>
                <span class="rounded-full bg-emerald-400/15 px-2 py-1 text-[11px] font-semibold text-emerald-200">{{ billingLabel(item.billingMode) }}</span>
              </div>
              <h2 class="mt-3 break-words text-xl font-black text-white">{{ item.model }}</h2>
              <p class="mt-1 text-xs text-slate-400">{{ item.channel }}</p>
            </div>
            <div class="rounded-2xl border border-cyan-300/20 bg-cyan-300/10 px-3 py-2 text-right">
              <div class="text-[10px] uppercase tracking-widest text-cyan-100">{{ t('modelSquare.effectiveRate') }}</div>
              <div class="text-lg font-black text-cyan-50">{{ item.effectiveRate }}x</div>
            </div>
          </div>

          <div class="mt-4 flex flex-wrap gap-2">
            <span v-for="group in item.groups" :key="group.id" class="rounded-full border border-white/10 bg-white/[0.07] px-2.5 py-1 text-xs text-slate-200">
              {{ group.name }} · {{ effectiveRate(group).toFixed(3).replace(/\.?0+$/, '') }}x
            </span>
          </div>

          <div v-if="item.prices.length > 0" class="mt-5 grid gap-2">
            <div v-for="price in item.prices" :key="price.label" class="rounded-2xl bg-black/25 p-3">
              <div class="flex items-center justify-between text-xs text-slate-400">
                <span>{{ price.label }}</span>
                <span>{{ price.unit }}</span>
              </div>
              <div class="mt-1 flex items-end justify-between gap-3">
                <span class="font-mono text-sm text-slate-300">${{ price.raw }}</span>
                <span class="font-mono text-2xl font-black text-emerald-200">${{ price.calculated }}</span>
              </div>
            </div>
          </div>
          <div v-else class="mt-5 rounded-2xl border border-dashed border-white/10 p-4 text-center text-sm text-slate-400">
            {{ t('modelSquare.noPricing') }}
          </div>

          <details v-if="item.intervalCount > 0" class="mt-4 text-xs text-slate-300">
            <summary class="cursor-pointer text-cyan-200">{{ t('modelSquare.intervalPricing', { count: item.intervalCount }) }}</summary>
            <p class="mt-2 text-slate-400">{{ t('modelSquare.intervalHint') }}</p>
          </details>
        </article>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import userChannelsAPI, { type UserAvailableGroup, type UserSupportedModelPricing } from '@/api/channels'
import userGroupsAPI from '@/api/groups'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { applyRateMultiplier, formatPrice, resolveEffectiveRate, toDisplayTokenPrice } from '@/utils/model-pricing'
import type { BillingMode } from '@/constants/channel'

interface ModelCard {
  key: string
  model: string
  platform: string
  channel: string
  groups: UserAvailableGroup[]
  billingMode: BillingMode
  prices: PriceLine[]
  effectiveRate: string
  intervalCount: number
  searchText: string
}

interface PriceLine {
  label: string
  raw: string
  calculated: string
  unit: string
}

const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(false)
const cards = ref<ModelCard[]>([])
const userGroupRates = ref<Record<number, number>>({})
const searchQuery = ref('')
const platformFilter = ref('')
const billingFilter = ref('')
const groupFilter = ref('')

const stats = computed(() => [
  { label: t('modelSquare.stats.models'), value: new Set(cards.value.map((item) => item.model)).size },
  { label: t('modelSquare.stats.platforms'), value: platforms.value.length },
  { label: t('modelSquare.stats.groups'), value: groupOptions.value.length },
])
const platforms = computed(() => [...new Set(cards.value.map((item) => item.platform))].sort())
const groupOptions = computed(() => {
  const groups = new Map<number, { id: number; name: string; rate: number }>()
  for (const card of cards.value) {
    for (const group of card.groups) {
      groups.set(group.id, { id: group.id, name: group.name, rate: effectiveRate(group) })
    }
  }
  return [...groups.values()].sort((a, b) => a.name.localeCompare(b.name))
})
const filteredModels = computed(() => {
  const query = searchQuery.value.trim().toLowerCase()
  return cards.value.filter((item) => matchesFilters(item, query))
})

function matchesFilters(item: ModelCard, query: string): boolean {
  if (query && !item.searchText.includes(query)) return false
  if (platformFilter.value && item.platform !== platformFilter.value) return false
  if (billingFilter.value && item.billingMode !== billingFilter.value) return false
  if (groupFilter.value && !item.groups.some((group) => String(group.id) === groupFilter.value)) return false
  return true
}

function effectiveRate(group: UserAvailableGroup): number {
  return resolveEffectiveRate({ groupId: group.id, groupRate: group.rate_multiplier, userGroupRates: userGroupRates.value })
}

function minEffectiveRate(groups: UserAvailableGroup[]): number {
  if (groups.length === 0) return 1
  return Math.min(...groups.map((group) => effectiveRate(group)))
}

function billingLabel(mode: BillingMode): string {
  if (mode === 'per_request') return t('modelSquare.billing.perRequest')
  if (mode === 'image') return t('modelSquare.billing.image')
  return t('modelSquare.billing.token')
}

function tokenPrice(value: number | null | undefined, rate: number): { raw: string; calculated: string } | null {
  const display = toDisplayTokenPrice(value)
  if (display === null) return null
  return { raw: formatPrice(display), calculated: formatPrice(applyRateMultiplier(display, rate)) }
}

function directPrice(value: number | null | undefined, rate: number): { raw: string; calculated: string } | null {
  const calculated = applyRateMultiplier(value, rate)
  if (calculated === null) return null
  return { raw: formatPrice(value), calculated: formatPrice(calculated) }
}

function buildPrices(pricing: UserSupportedModelPricing | null, rate: number): PriceLine[] {
  if (!pricing) return []
  const unit = pricing.billing_mode === 'token' ? t('modelSquare.units.perMillion') : t('modelSquare.units.perRequest')
  if (pricing.billing_mode !== 'token') {
    const price = directPrice(pricing.per_request_price, rate)
    return price ? [{ label: t('modelSquare.price.perRequest'), raw: price.raw, calculated: price.calculated, unit }] : []
  }
  return [
    makePriceLine(t('modelSquare.price.input'), pricing.input_price, rate, unit),
    makePriceLine(t('modelSquare.price.output'), pricing.output_price, rate, unit),
    makePriceLine(t('modelSquare.price.cacheWrite'), pricing.cache_write_price, rate, unit),
    makePriceLine(t('modelSquare.price.cacheRead'), pricing.cache_read_price, rate, unit),
    makePriceLine(t('modelSquare.price.imageOutput'), pricing.image_output_price, rate, unit),
  ].filter((line): line is PriceLine => line !== null)
}

function makePriceLine(label: string, value: number | null | undefined, rate: number, unit: string): PriceLine | null {
  const price = tokenPrice(value, rate)
  return price ? { label, raw: price.raw, calculated: price.calculated, unit } : null
}

function buildSearchText(channel: string, model: string, platform: string, groups: UserAvailableGroup[]): string {
  return [channel, model, platform, ...groups.map((group) => group.name)].join(' ').toLowerCase()
}

async function loadModels() {
  loading.value = true
  try {
    const [channels, rates] = await Promise.all([userChannelsAPI.getAvailable(), userGroupsAPI.getUserGroupRates()])
    userGroupRates.value = rates
    cards.value = channels.flatMap((channel) =>
      channel.platforms.flatMap((section) =>
        section.supported_models.map((model) => {
          const rate = minEffectiveRate(section.groups)
          return {
            key: `${channel.name}:${section.platform}:${model.name}`,
            model: model.name,
            platform: section.platform,
            channel: channel.name,
            groups: section.groups,
            billingMode: model.pricing?.billing_mode ?? 'token',
            prices: buildPrices(model.pricing, rate),
            effectiveRate: rate.toFixed(3).replace(/\.?0+$/, ''),
            intervalCount: model.pricing?.intervals?.length ?? 0,
            searchText: buildSearchText(channel.name, model.name, section.platform, section.groups),
          }
        }),
      ),
    )
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('modelSquare.loadFailed')))
  } finally {
    loading.value = false
  }
}

onMounted(loadModels)
</script>

<style scoped>
.model-square {
  background:
    radial-gradient(circle at top left, rgba(34, 211, 238, 0.35), transparent 34rem),
    radial-gradient(circle at 85% 15%, rgba(16, 185, 129, 0.25), transparent 30rem),
    linear-gradient(135deg, #07111f 0%, #0e1b2f 45%, #111827 100%);
}

.model-input {
  width: 100%;
  border-radius: 1rem;
  border: 1px solid rgba(255, 255, 255, 0.12);
  background: rgba(15, 23, 42, 0.75);
  padding: 0.75rem 1rem;
  color: white;
  outline: none;
}

.model-card {
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 1.5rem;
  background: linear-gradient(145deg, rgba(255, 255, 255, 0.12), rgba(15, 23, 42, 0.72));
  padding: 1.25rem;
  box-shadow: 0 1.5rem 4rem rgba(0, 0, 0, 0.28);
  backdrop-filter: blur(18px);
}
</style>
