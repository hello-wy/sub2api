<template>
  <div class="plaza-gallery" :class="{ 'plaza-embedded': embedded }">
    <header class="plaza-page-title">
      <span class="plaza-eyebrow">MODEL LIBRARY</span>
      <h1>{{ t('modelPlaza.title') }}</h1>
      <p>{{ t('modelPlaza.gallery.catalogDescription') }}</p>
      <div v-if="!loading && !error" class="catalog-stats">
        <span><strong>{{ modelCount(entries) }}</strong> {{ t('modelPlaza.gallery.modelsLabel') }}</span>
        <span><strong>{{ providerOptions.length - 1 }}</strong> {{ t('modelPlaza.gallery.providers') }}</span>
        <span><strong>{{ nonEmptyGroups.length }}</strong> {{ t('modelPlaza.gallery.groupsLabel') }}</span>
      </div>
    </header>
    <div class="plaza-workspace">
      <aside class="plaza-filters">
        <div class="filters-heading">
          <h2 class="desktop-filter-title">{{ t('modelPlaza.gallery.filters') }}</h2>
          <button type="button" class="filters-toggle" :aria-expanded="filtersOpen" aria-controls="plaza-filter-groups" @click="filtersOpen = !filtersOpen">
            {{ t('modelPlaza.gallery.filters') }}<Icon name="chevronDown" size="sm" class="mobile-chevron" :class="{ 'rotate-180': filtersOpen }" />
          </button>
          <button type="button" class="reset-filters" @click="reset">{{ t('modelPlaza.gallery.reset') }}</button>
        </div>
        <div id="plaza-filter-groups" :class="{ 'filters-closed': !filtersOpen }">
          <PlazaFilterSection v-model="platform" :title="t('modelPlaza.gallery.providers')" :options="providerOptions" />
          <PlazaFilterSection v-model="groupId" :title="t('modelPlaza.gallery.groups')" :options="groupOptions">
            <label v-if="rates.length > 1" class="rate-filter">
              <span>{{ t('modelPlaza.filters.rateLabel') }}</span>
              <select v-model="rate"><option value="all">{{ t('modelPlaza.filters.all') }}</option><option v-for="value in rates" :key="value" :value="String(value)">{{ value }}×</option></select>
            </label>
          </PlazaFilterSection>
          <PlazaFilterSection v-model="billing" :title="t('modelPlaza.gallery.billing')" :options="billingOptions" />
        </div>
      </aside>

      <section class="plaza-results" :aria-label="t('modelPlaza.title')" :aria-busy="loading">
        <div class="plaza-catalog-header">
          <PlazaProviderBanner
            :platform="platform"
            :title="platform === 'all' ? t('modelPlaza.gallery.allModels') : platformLabel(platform)"
            :count="filtered.length"
            :loading="loading"
            :error="error"
          />
          <div class="plaza-toolbar">
            <label class="plaza-search">
              <Icon name="search" size="md" />
              <input v-model="search" type="search" :placeholder="t('modelPlaza.gallery.search')" :aria-label="t('modelPlaza.filters.searchPlaceholder')" />
            </label>
            <label class="sort-control">
              <span class="sr-only">{{ t('modelPlaza.gallery.sortLabel') }}</span>
              <select v-model="sort"><option value="name">{{ t('modelPlaza.gallery.sortName') }}</option><option value="price">{{ t('modelPlaza.gallery.sortPrice') }}</option></select>
            </label>
            <div class="price-switch" role="group" :aria-label="t('modelPlaza.gallery.priceMode')">
              <button v-for="mode in (['paid', 'official'] as const)" :key="mode" type="button" :aria-pressed="priceMode === mode" :class="{ active: priceMode === mode }" @click="priceMode = mode">{{ t('modelPlaza.gallery.' + mode) }}</button>
            </div>
            <button class="billing-help-button" type="button" aria-haspopup="dialog" @click="showHelp = true"><Icon name="infoCircle" size="xs" /> {{ t('modelPlaza.gallery.help') }}</button>
            <div class="view-switch" role="group" :aria-label="t('modelPlaza.gallery.view')">
              <button v-for="mode in (['grid', 'list'] as const)" :key="mode" type="button" :class="{ active: view === mode }" :aria-pressed="view === mode" :aria-label="t('modelPlaza.gallery.' + mode)" :title="t('modelPlaza.gallery.' + mode)" @click="view = mode">
                <Icon :name="mode" size="sm" /><span>{{ t('modelPlaza.gallery.' + mode + 'Label') }}</span>
              </button>
            </div>
          </div>
        </div>
        <div v-if="activeFilters.length" class="active-filters">
          <button v-for="filter in activeFilters" :key="filter.key" type="button" @click="filter.clear()" :aria-label="t('modelPlaza.gallery.removeFilter', { label: filter.label })">{{ filter.label }}<Icon name="x" size="xs" /></button>
        </div>
        <PlazaBillingGuide :show="showHelp" :entries="entries" @close="showHelp = false">
          <div v-if="descriptionHtml" class="plaza-description" v-html="descriptionHtml"></div>
        </PlazaBillingGuide>
        <p v-if="!isAuthenticated" class="plaza-notice">{{ t('modelPlaza.anonymousHint') }} · {{ t('modelPlaza.gallery.loginStatus') }}</p>
        <p v-else-if="monitor.failed.value" class="plaza-notice" role="status">{{ t('modelPlaza.gallery.statusFailed') }}</p>


        <div v-if="loading" class="plaza-card-grid" aria-live="polite">
          <span class="sr-only">{{ t('modelPlaza.loading') }}</span>
          <div v-for="n in 6" :key="n" class="model-skeleton"><div /><div /><div /></div>
        </div>
        <div v-else-if="error" class="plaza-empty" role="alert">
          <Icon name="infoCircle" size="lg" /><p>{{ t('modelPlaza.loadFailed') }}</p>
          <button type="button" @click="$emit('retry')">{{ t('modelPlaza.gallery.retry') }}</button>
        </div>
        <template v-else-if="filtered.length">
          <div class="plaza-card-grid" :class="{ 'list-view': view === 'list' }">
            <PlazaModelCard v-for="item in pageItems" :key="item.key" :model="item.model" :group="item.group" :variants="item.variants" :price-mode="priceMode" :view="view" :status="item.status" :show-throughput="true" @details="openDetails(item, $event)" />
          </div>
          <Pagination v-if="filtered.length > pageSize" v-model:page="page" class="plaza-pagination" :total="filtered.length" :page-size="pageSize" :show-page-size-selector="false" />
        </template>
        <div v-else class="plaza-empty">
          <Icon name="search" size="lg" /><p>{{ t('modelPlaza.noSearchResult') }}</p>
          <button type="button" @click="reset">{{ t('modelPlaza.gallery.reset') }}</button>
        </div>
      </section>
    </div>
    <PlazaModelDetails :variants="detailVariants" :selected-key="detailKey" :price-mode="priceMode" :show-throughput="true" @close="detailVariantKeys = []" />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'
import PlazaFilterSection, { type PlazaFilterOption } from './PlazaFilterSection.vue'
import PlazaModelCard from './PlazaModelCard.vue'
import PlazaProviderBanner from './PlazaProviderBanner.vue'
import PlazaModelDetails from './PlazaModelDetails.vue'
import './plaza.css'
import PlazaBillingGuide from './PlazaBillingGuide.vue'
import { useAuthStore } from '@/stores/auth'
import { usePlazaStatus } from '@/composables/usePlazaStatus'
import { plazaRate, plazaPrices, type PlazaPriceMode, type PlazaModelVariant } from '@/utils/model-plaza'
import { platformLabel } from '@/utils/platformColors'
import type { ModelPlazaResponse } from '@/api/modelPlaza'

const props = defineProps<{ response: ModelPlazaResponse | null; loading: boolean; error?: boolean; embedded?: boolean }>()
defineEmits<{ retry: [] }>()
const { t } = useI18n()
const auth = useAuthStore()
const isAuthenticated = computed(() => auth.isAuthenticated)
const monitor = usePlazaStatus()
const platform = ref('all'), groupId = ref('all'), rate = ref('all'), billing = ref('all')
const search = ref(''), view = ref<'grid' | 'list'>('grid'), priceMode = ref<PlazaPriceMode>('paid')
const filtersOpen = ref(false), showHelp = ref(false), page = ref(1)
const pageSize = 12
const sort = ref<'name' | 'price'>('name')
const detailVariantKeys = ref<string[]>([])
const detailVariants = computed(() => detailVariantKeys.value.map(key => entries.value.find(entry => entry.key === key)).filter((entry): entry is PlazaEntry => !!entry))
const detailKey = ref('')
function openDetails(item: DisplayEntry, key: string) { detailVariantKeys.value = item.variants.map(variant => variant.key); detailKey.value = key }
const descriptionHtml = computed(() => {
  const md = props.response?.description?.trim()
  return md ? DOMPurify.sanitize(marked.parse(md) as string) : ''
})
type PlazaEntry = PlazaModelVariant & { billing: string }
const entries = computed<PlazaEntry[]>(() => (props.response?.groups ?? []).flatMap(group => group.models.map(model => ({
  key: JSON.stringify([group.id, model.platform, model.name]), group, model,
  billing: model.pricing?.billing_mode ?? 'token',
  status: monitor.statusFor(model, group),
}))))
const modelIdentity = (entry: PlazaEntry) => `${entry.model.platform.trim().normalize('NFKC').toLowerCase()}\u0000${entry.model.name.trim().normalize('NFKC').toLowerCase()}`
const modelCount = (items: PlazaEntry[]) => new Set(items.map(modelIdentity)).size
const nonEmptyGroups = computed(() => (props.response?.groups ?? []).filter(group => group.models.length))
type DisplayEntry = PlazaEntry & { variants: PlazaEntry[] }
function entryPrice(entry: PlazaEntry) {
  const prices = plazaPrices(entry.model, entry.group, priceMode.value)[0]
  return prices.request ?? prices.input ?? prices.output ?? Number.POSITIVE_INFINITY
}
function representative(a: PlazaEntry, b: PlazaEntry) {
  return entryPrice(a) - entryPrice(b) || plazaRate(a.group, a.model) - plazaRate(b.group, b.model)
    || a.group.name.localeCompare(b.group.name) || a.key.localeCompare(b.key)
}
const visibleEntries = computed<DisplayEntry[]>(() => {
  const q = search.value.trim().toLowerCase()
  const matching = entries.value.filter(e =>
    (platform.value === 'all' || e.model.platform === platform.value)
    && (groupId.value === 'all' || String(e.group.id) === groupId.value)
    && (billing.value === 'all' || e.billing === billing.value)
    && (rate.value === 'all' || String(plazaRate(e.group)) === rate.value)
    && (!q || [e.model.name, e.group.name, platformLabel(e.model.platform)].some(v => v.toLowerCase().includes(q))))
  if (groupId.value !== 'all') return matching.map(item => ({ ...item, variants: [item] }))
  const grouped = new Map<string, PlazaEntry[]>()
  for (const item of matching) {
    const identity = modelIdentity(item)
    grouped.set(identity, [...(grouped.get(identity) ?? []), item])
  }
  return [...grouped.values()].map(items => {
    const sorted = [...items].sort(representative)
    return { ...sorted[0], key: `model:${modelIdentity(sorted[0])}`, variants: sorted }
  })
})
const allOption = (label: string, count = modelCount(entries.value)): PlazaFilterOption => ({ value:'all', label:t('modelPlaza.gallery.' + label), badge:count })
const providerOptions = computed(() => [allOption('allProviders'), ...[...new Set(entries.value.map(e => e.model.platform))].sort().map(value => ({
  value, label:platformLabel(value), platform:value, badge:modelCount(entries.value.filter(e => e.model.platform === value)),
}))])
const providerEntries = computed(() => entries.value.filter(e => platform.value === 'all' || e.model.platform === platform.value))
const availableGroups = computed(() => {
  const ids = new Set(providerEntries.value.map(e => e.group.id))
  return (props.response?.groups ?? []).filter(g => ids.has(g.id))
})
const groupOptions = computed(() => [allOption('allGroups', modelCount(providerEntries.value)), ...availableGroups.value.map(g => ({ value:String(g.id), label:g.name, badge:'×' + plazaRate(g) }))])
const billingOptions = computed(() => [allOption('allBilling'), ...[...new Set(entries.value.map(e => e.billing))].map(value => ({ value, label:t('modelPlaza.gallery.billingModes.' + value), badge:modelCount(entries.value.filter(e => e.billing === value)) }))])
const rates = computed(() => [...new Set(availableGroups.value.map(g => plazaRate(g)))].sort((a,b) => a-b))
const filtered = computed(() => [...visibleEntries.value].sort((a,b) =>
  (sort.value === 'price' ? representative(a, b) : 0)
  || a.model.name.localeCompare(b.model.name, undefined, { numeric: true }) || a.model.platform.localeCompare(b.model.platform)))
const activeFilters = computed(() => {
  const sections = [
    { key: 'platform', selected: platform, options: providerOptions },
    { key: 'group', selected: groupId, options: groupOptions },
    { key: 'billing', selected: billing, options: billingOptions },
  ]
  const chips = sections.filter(section => section.selected.value !== 'all').map(section => ({
    key: section.key, label: section.options.value.find(option => option.value === section.selected.value)?.label ?? section.selected.value,
    clear: () => { section.selected.value = 'all' },
  }))
  if (rate.value !== 'all') chips.push({ key: 'rate', label: `${rate.value}×`, clear: () => { rate.value = 'all' } })
  if (search.value.trim()) chips.push({ key: 'search', label: search.value.trim(), clear: () => { search.value = '' } })
  return chips
})
const pageItems = computed(() => filtered.value.slice((page.value - 1) * pageSize,page.value * pageSize))
watch([search,platform,groupId,rate,billing,priceMode,sort], () => { page.value = 1 })
watch(() => filtered.value.length, count => { page.value = Math.min(page.value, Math.max(1,Math.ceil(count/pageSize))) })
watch([providerOptions,groupOptions,billingOptions,rates], () => {
  for (const [selected, options] of [[platform,providerOptions],[groupId,groupOptions],[billing,billingOptions]] as const) {
    if (!options.value.some(o => o.value === selected.value)) selected.value = 'all'
  }
  if (rate.value !== 'all' && !rates.value.includes(Number(rate.value))) rate.value = 'all'
})
function reset() { platform.value = groupId.value = rate.value = billing.value = 'all'; search.value = ''; page.value = 1 }
</script>

<style scoped>
.plaza-page-title { padding:1.25rem 1rem 2.25rem; text-align:center; }
.plaza-eyebrow { display:block; margin-bottom:.8rem; font-family:var(--plaza-numbers); color:var(--plaza-blue); font-size:.62rem; font-weight:500; letter-spacing:.18em; }
.plaza-page-title h1 { font-size:clamp(1.75rem,3vw,2.65rem); line-height:1.2; font-weight:700; letter-spacing:-.045em; }
.plaza-page-title>p { margin:.85rem auto 0; max-width:38rem; color:var(--plaza-muted); font-size:.85rem; line-height:1.8; }
.catalog-stats { display:flex; flex-wrap:wrap; align-items:center; justify-content:center; gap:.6rem 1.3rem; margin-top:.9rem; color:var(--plaza-muted); font-size:.7rem; }
.catalog-stats>span+span { border-left:1px solid var(--plaza-line); padding-left:1.3rem; }
.catalog-stats strong { color:var(--plaza-ink); font-family:var(--plaza-numbers); font-weight:600; margin-right:.2rem; }
.plaza-search { display:flex; flex:1 1 16rem; align-items:center; gap:.5rem; min-width:0; padding:0 .75rem; height:2.25rem; border-radius:.65rem; border:1px solid transparent; background:var(--plaza-subtle); color:var(--plaza-muted); }
.plaza-search input { width:100%; min-width:0; height:100%; border:0; background:transparent; font-size:.8rem; color:var(--plaza-ink); outline:none; box-shadow:none; padding:0; }
.plaza-search:focus-within { border-color:var(--plaza-blue); outline:3px solid var(--plaza-selected); }
.plaza-workspace { display:grid; grid-template-columns:18.5rem minmax(0,1fr); gap:1.5rem; align-items:start; }
.plaza-filters { min-width:0; padding:1rem .85rem 1.25rem; border:1px solid var(--plaza-line); border-radius:1rem; background:var(--plaza-surface); }
.filters-heading { display:flex; align-items:center; justify-content:space-between; padding-bottom:1.1rem; border-bottom:1px solid var(--plaza-soft-line); }
.desktop-filter-title { font-size:1rem; font-weight:650; }
.filters-toggle { display:none; align-items:center; gap:.5rem; color:var(--plaza-ink); font-size:.85rem; font-weight:600; }
.reset-filters { font-size:.75rem; color:var(--plaza-muted); } .reset-filters:hover { color:var(--plaza-blue); }
.mobile-chevron { display:none; }
.rate-filter { display:flex; gap:.5rem; align-items:center; justify-content:space-between; margin-top:.75rem; font-size:.7rem; color:var(--plaza-muted); }
.rate-filter select { max-width:65%; padding:.3rem 1.7rem .3rem .5rem; border-radius:.4rem; border:1px solid var(--plaza-line); background-color:var(--plaza-surface); color:var(--plaza-ink); font-size:.7rem; }
.plaza-results { container:plaza-results / inline-size; min-width:0; }
.plaza-catalog-header { margin-bottom:1.4rem; border:1px solid var(--plaza-line); border-radius:.9rem; overflow:hidden; background:var(--plaza-surface); box-shadow:0 2px 3px rgb(15 23 42 / .025); }
.plaza-toolbar { display:flex; flex-wrap:wrap; gap:.5rem; align-items:center; padding:.65rem; }
.sort-control select { height:2.25rem; max-width:10rem; padding:.35rem 1.7rem .35rem .6rem; border:1px solid var(--plaza-line); border-radius:.5rem; background-color:var(--plaza-surface); color:var(--plaza-muted); font-size:.7rem; }
.price-switch,.view-switch { display:flex; flex-shrink:0; height:2.25rem; align-items:center; padding:.2rem; gap:.15rem; background:var(--plaza-surface); border:1px solid var(--plaza-line); border-radius:.5rem; }
.price-switch button,.view-switch button { display:flex; align-items:center; justify-content:center; gap:.3rem; height:100%; padding:0 .55rem; font-size:.7rem; border-radius:.3rem; color:var(--plaza-muted); white-space:nowrap; }
.price-switch button.active,.view-switch button.active { background:var(--plaza-selected); color:var(--plaza-blue); font-weight:600; }
.view-switch button { padding:0 .45rem; }
.view-switch button span { display:none; }
.billing-help-button { display:flex; align-items:center; justify-content:center; flex-shrink:0; gap:.3rem; height:2.25rem; padding:0 .7rem; white-space:nowrap; border:1px solid var(--plaza-line); border-radius:.5rem; color:var(--plaza-muted); background:var(--plaza-surface); font-size:.7rem; }
.billing-help-button:hover { color:var(--plaza-blue); border-color:var(--plaza-hover-line); }
.active-filters { display:flex; flex-wrap:wrap; gap:.5rem; margin:0 0 1rem; }
.active-filters button { display:flex; align-items:center; gap:.4rem; max-width:100%; padding:.3rem .55rem; border:1px solid var(--plaza-line); border-radius:.4rem; background:var(--plaza-surface); color:var(--plaza-muted); font-size:.65rem; overflow-wrap:anywhere; }
.active-filters button:hover { border-color:var(--plaza-hover-line); color:var(--plaza-blue); }
.plaza-description { margin-top:.75rem; }
.plaza-notice { margin:0 0 1rem; font-size:.7rem; color:var(--plaza-muted); line-height:1.7; }
.plaza-card-grid { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:1rem; align-items:stretch; }
.plaza-card-grid.list-view { grid-template-columns:1fr; gap:.8rem; }
.plaza-empty { min-height:24rem; display:flex; flex-direction:column; align-items:center; justify-content:center; gap:1rem; border:1px dashed var(--plaza-line); border-radius:1rem; color:var(--plaza-muted); font-size:.85rem; }
.plaza-empty button { padding:.6rem 1rem; border:1px solid var(--plaza-line); background:var(--plaza-surface); border-radius:.5rem; color:var(--plaza-blue); }
.plaza-pagination { margin-top:1.4rem; }
.model-skeleton { height:22rem; padding:1.25rem; border:1px solid var(--plaza-line); border-radius:1rem; background:var(--plaza-surface); }
.model-skeleton>div { width:70%; height:2rem; margin-bottom:2rem; border-radius:.5rem; background:var(--plaza-subtle); animation:plaza-pulse 1.5s ease-in-out infinite; }
.model-skeleton>div:nth-child(2) { width:100%; height:8rem; } .model-skeleton>div:nth-child(3) { height:1rem; width:85%; }
button:focus-visible,select:focus-visible { outline:2px solid var(--plaza-blue); outline-offset:3px; }
@keyframes plaza-pulse { 50% { opacity:.4; } }
@container plaza-results (max-width:52rem) { .plaza-card-grid:not(.list-view) { grid-template-columns:repeat(2,minmax(0,1fr)); } }
@container plaza-results (max-width:34rem) { .plaza-card-grid:not(.list-view) { grid-template-columns:1fr; } }
@media (max-width:1100px) { .plaza-workspace { grid-template-columns:16rem minmax(0,1fr); gap:1rem; } .plaza-filters { padding:1rem .5rem; } }
@media (max-width:767px) { .plaza-workspace { grid-template-columns:minmax(0,1fr); } .plaza-filters { padding:1rem; border:1px solid var(--plaza-line); border-radius:.8rem; } .filters-closed { display:none; } .mobile-chevron { display:block; } .filters-heading { padding-bottom:0; border:0; } .filters-toggle { display:flex; } .desktop-filter-title { display:none; } .plaza-page-title { padding:.5rem 0 1.4rem; } .plaza-eyebrow { font-size:.6rem; } .plaza-page-title>p { font-size:.75rem; } .plaza-search { flex-basis:100%; height:2.5rem; } .plaza-toolbar { gap:.4rem; } .sort-control { margin-left:auto; } .price-switch { margin-right:auto; } .plaza-card-grid { gap:.8rem; } }
@media (prefers-reduced-motion:reduce) { .model-skeleton>div { animation:none; } }
.plaza-description {
  line-height: 1.7;
  overflow-wrap: anywhere;
}

.plaza-description :deep(h1),
.plaza-description :deep(h2),
.plaza-description :deep(h3) {
  @apply mb-2 mt-3 font-semibold text-gray-900 first:mt-0 dark:text-white;
}

.plaza-description :deep(p) {
  @apply mb-2 text-gray-700 last:mb-0 dark:text-dark-200;
}

.plaza-description :deep(a) {
  @apply text-primary-600 underline underline-offset-4 hover:text-primary-700 dark:text-primary-300;
}

.plaza-description :deep(ul) {
  @apply mb-2 list-disc pl-5;
}

.plaza-description :deep(ol) {
  @apply mb-2 list-decimal pl-5;
}

.plaza-description :deep(li) {
  @apply mb-0.5 text-gray-700 dark:text-dark-200;
}

.plaza-description :deep(code) {
  @apply rounded bg-gray-100 px-1.5 py-0.5 font-mono text-xs dark:bg-dark-800;
}

.plaza-description :deep(blockquote) {
  @apply my-2 border-l-4 border-gray-300 pl-3 text-gray-600 dark:border-dark-600 dark:text-dark-300;
}

</style>
