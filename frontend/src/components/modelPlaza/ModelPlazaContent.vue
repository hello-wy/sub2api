<template>
  <div class="plaza-gallery">
    <header v-if="!embedded" class="plaza-page-title">
      <h1>{{ t('modelPlaza.title') }}</h1>
      <p>{{ t('modelPlaza.description') }}</p>
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
          <PlazaFilterSection v-model="tag" :title="t('modelPlaza.gallery.tags')" :options="tagOptions" />
          <PlazaFilterSection v-model="groupId" :title="t('modelPlaza.gallery.groups')" :options="groupOptions">
            <label v-if="rates.length > 1" class="rate-filter">
              <span>{{ t('modelPlaza.filters.rateLabel') }}</span>
              <select v-model="rate"><option value="all">{{ t('modelPlaza.filters.all') }}</option><option v-for="value in rates" :key="value" :value="String(value)">{{ value }}×</option></select>
            </label>
          </PlazaFilterSection>
          <PlazaFilterSection v-model="billing" :title="t('modelPlaza.gallery.billing')" :options="billingOptions" />
          <PlazaFilterSection v-model="health" :title="t('modelPlaza.gallery.channelStatus')" :options="healthOptions" />
        </div>
      </aside>

      <section class="plaza-results" :aria-label="t('modelPlaza.title')" :aria-busy="loading">
        <div class="plaza-toolbar">
          <label class="plaza-search">
            <Icon name="search" size="sm" />
            <input v-model="search" type="search" :placeholder="t('modelPlaza.gallery.search')" :aria-label="t('modelPlaza.filters.searchPlaceholder')" />
            <span class="result-count" aria-live="polite" :title="t('modelPlaza.gallery.allModels')"><span class="sr-only">{{ t('modelPlaza.gallery.allModels') }}</span>{{ filtered.length }}</span>
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
            <PlazaModelCard v-for="item in pageItems" :key="item.key" :model="item.model" :group="item.group" :price-mode="priceMode" :view="view" :status="item.status" :show-throughput="showThroughput" />
          </div>
          <Pagination v-if="filtered.length > pageSize" v-model:page="page" class="plaza-pagination" :total="filtered.length" :page-size="pageSize" :show-page-size-selector="false" />
        </template>
        <div v-else class="plaza-empty">
          <Icon name="search" size="lg" /><p>{{ t('modelPlaza.noSearchResult') }}</p>
          <button type="button" @click="reset">{{ t('modelPlaza.gallery.reset') }}</button>
        </div>
      </section>
    </div>
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
import PlazaBillingGuide from './PlazaBillingGuide.vue'
import { useAuthStore } from '@/stores/auth'
import { usePlazaStatus } from '@/composables/usePlazaStatus'
import { isChannelMonitorThroughputHidden } from '@/utils/featureFlags'
import { plazaRate, type PlazaPriceMode } from '@/utils/model-plaza'
import { platformLabel } from '@/utils/platformColors'
import type { ModelPlazaResponse, PlazaModel, ModelPlazaGroup } from '@/api/modelPlaza'

const props = defineProps<{ response: ModelPlazaResponse | null; loading: boolean; error?: boolean; embedded?: boolean }>()
defineEmits<{ retry: [] }>()
const { t } = useI18n()
const auth = useAuthStore()
const isAuthenticated = computed(() => auth.isAuthenticated)
const monitor = usePlazaStatus()
const showThroughput = computed(() => !isChannelMonitorThroughputHidden())
const platform = ref('all'), groupId = ref('all'), tag = ref('all'), rate = ref('all'), billing = ref('all'), health = ref('all')
const search = ref(''), view = ref<'grid' | 'list'>('grid'), priceMode = ref<PlazaPriceMode>('paid')
const filtersOpen = ref(false), showHelp = ref(false), page = ref(1)
const pageSize = 12
const descriptionHtml = computed(() => {
  const md = props.response?.description?.trim()
  return md ? DOMPurify.sanitize(marked.parse(md) as string) : ''
})
function tags(model: PlazaModel, group: ModelPlazaGroup) {
  return [
    ...(model.pricing?.intervals && model.pricing.intervals.length > 1 ? ['tiered'] : []),
    ...(model.time_pricing?.periods.length ? ['timePricing'] : []),
    ...(group.is_exclusive ? ['exclusive'] : []),
    ...(group.subscription_type === 'subscription' ? ['subscription'] : []),
  ]
}
const entries = computed(() => (props.response?.groups ?? []).flatMap(group => group.models.map(model => ({
  key: JSON.stringify([group.id, model.platform, model.name]), group, model,
  tags: tags(model, group), billing: model.pricing?.billing_mode ?? 'token',
  status: monitor.statusFor(model, group),
}))))
const allOption = (label: string, count = entries.value.length): PlazaFilterOption => ({ value:'all', label:t('modelPlaza.gallery.' + label), badge:count })
const providerOptions = computed(() => [allOption('allProviders'), ...[...new Set(entries.value.map(e => e.model.platform))].sort().map(value => ({
  value, label:platformLabel(value), platform:value, badge:entries.value.filter(e => e.model.platform === value).length,
}))])
const groupOptions = computed(() => [allOption('allGroups'), ...(props.response?.groups ?? []).filter(g => g.models.length).map(g => ({ value:String(g.id), label:g.name, badge:'×' + plazaRate(g) }))])
const tagOptions = computed(() => [allOption('allTags'), ...['tiered','timePricing','exclusive','subscription'].filter(value => entries.value.some(e => e.tags.includes(value))).map(value => ({ value, label:t('modelPlaza.gallery.' + value), badge:entries.value.filter(e => e.tags.includes(value)).length }))])
const billingOptions = computed(() => [allOption('allBilling'), ...[...new Set(entries.value.map(e => e.billing))].map(value => ({ value, label:t('modelPlaza.gallery.billingModes.' + value), badge:entries.value.filter(e => e.billing === value).length }))])
const healthOptions = computed(() => [allOption('allStatus'), ...['healthy','warning','critical','unknown'].map(value => ({ value, label:t('modelPlaza.gallery.health.' + value), badge:entries.value.filter(e => e.status.health === value).length }))])
const rates = computed(() => [...new Set((props.response?.groups ?? []).map(g => plazaRate(g)))].sort((a,b) => a-b))
const filtered = computed(() => {
  const q = search.value.trim().toLowerCase()
  const list = entries.value.filter(e =>
    (platform.value === 'all' || e.model.platform === platform.value)
    && (groupId.value === 'all' || String(e.group.id) === groupId.value)
    && (tag.value === 'all' || e.tags.includes(tag.value))
    && (billing.value === 'all' || e.billing === billing.value)
    && (rate.value === 'all' || String(plazaRate(e.group)) === rate.value)
    && (health.value === 'all' || e.status.health === health.value)
    && (!q || [e.model.name, e.group.name, platformLabel(e.model.platform)].some(v => v.toLowerCase().includes(q))))
  return list.sort((a,b) => plazaRate(a.group) - plazaRate(b.group) || a.group.name.localeCompare(b.group.name) || a.model.name.localeCompare(b.model.name))
})
const pageItems = computed(() => filtered.value.slice((page.value - 1) * pageSize,page.value * pageSize))
watch([search,platform,groupId,tag,rate,billing,health,priceMode], () => { page.value = 1 })
watch(() => filtered.value.length, count => { page.value = Math.min(page.value, Math.max(1,Math.ceil(count/pageSize))) })
watch([providerOptions,groupOptions,tagOptions,billingOptions,rates], () => {
  for (const [selected, options] of [[platform,providerOptions],[groupId,groupOptions],[tag,tagOptions],[billing,billingOptions]] as const) {
    if (!options.value.some(o => o.value === selected.value)) selected.value = 'all'
  }
  if (rate.value !== 'all' && !rates.value.includes(Number(rate.value))) rate.value = 'all'
})
function reset() { platform.value = groupId.value = tag.value = rate.value = billing.value = health.value = 'all'; search.value = ''; page.value = 1 }
</script>

<style scoped>
.plaza-gallery { --plaza-ink:#263c56; --plaza-muted:#75869d; --plaza-blue:#1677ff; --plaza-line:#e1e7ef; --plaza-rail:#fff; --plaza-selected:#f0f3f7; --plaza-button:rgb(255 255 255 / .8); --plaza-glass:linear-gradient(135deg,rgb(255 255 255 / .82),rgb(255 255 255 / .58)); --plaza-glass-edge:rgb(255 255 255 / .92); color:var(--plaza-ink); }
.plaza-page-title { margin-bottom:1.5rem; } .plaza-page-title h1 { font-size:1.65rem; font-weight:700; letter-spacing:-.035em; } .plaza-page-title p { margin-top:.35rem; color:var(--plaza-muted); font-size:.85rem; }
.plaza-workspace { display:grid; grid-template-columns:20rem minmax(0,1fr); border:1px solid var(--plaza-glass-edge); border-radius:1.3rem; overflow:hidden; background:radial-gradient(ellipse at 38% 30%,rgb(171 204 249 / .43),transparent 65%),radial-gradient(ellipse at 100% 85%,rgb(173 225 229 / .33),transparent 60%),#f1f6fc; }
.plaza-filters { padding:1.4rem 1.25rem; background:var(--plaza-rail); }
.plaza-filters, .plaza-filters :deep(*) { @apply font-sans; }
.filters-heading { display:flex; align-items:center; justify-content:space-between; padding-bottom:1rem; border-bottom:1px solid var(--plaza-line); }
.desktop-filter-title { font-size:1.2rem; font-weight:650; color:var(--plaza-ink); }
.filters-toggle { display:none; align-items:center; gap:.5rem; color:var(--plaza-ink); font-size:1.2rem; font-weight:650; }
.reset-filters { font-size:.875rem; font-weight:500; color:var(--plaza-muted); } .reset-filters:hover { color:var(--plaza-blue); }
.mobile-chevron { display:none; }
.rate-filter { display:flex; gap:.5rem; align-items:center; justify-content:space-between; margin-top:.75rem; font-size:.8125rem; color:var(--plaza-muted); }
.rate-filter select { max-width:65%; padding:.3rem; border-radius:.4rem; border:1px solid var(--plaza-line); background:var(--plaza-button); }
.plaza-results { container:plaza-results / inline-size; padding:1.3rem; min-width:0; }
.plaza-toolbar { display:flex; gap:.5rem; align-items:center; margin-bottom:1rem; }
.plaza-search { display:flex; flex:1 1 10rem; min-width:0; gap:.6rem; align-items:center; padding:0 .85rem; height:2.5rem; border-radius:.75rem; border:1px solid var(--plaza-glass-edge); background:var(--plaza-button); color:var(--plaza-muted); }
.plaza-search input { width:100%; min-width:0; border:0; background:transparent; font-size:.8rem; color:var(--plaza-ink); outline:none; box-shadow:none; padding:0; }
.plaza-search:focus-within { outline:2px solid #1677ff; outline-offset:2px; }
.price-switch { display:flex; height:2.5rem; align-items:center; flex-shrink:0; padding:.25rem; background:var(--plaza-button); border:1px solid var(--plaza-glass-edge); border-radius:.75rem; }
.price-switch button { padding:.45rem .7rem; font-size:.76rem; border-radius:.5rem; color:var(--plaza-muted); font-weight:600; white-space:nowrap; }
.price-switch .active { background:#1677ff; color:white; box-shadow:0 3px 7px rgb(22 119 255 / .14); }
.billing-help-button { display:flex; align-items:center; justify-content:center; flex-shrink:0; gap:.4rem; height:2.5rem; padding:0 .85rem; white-space:nowrap; border:1px solid #c6dcff; border-radius:.7rem; color:#1265db; background:#fff; box-shadow:0 2px 7px rgb(22 119 255 / .06); font-size:.75rem; font-weight:600; cursor:pointer; transition:background .18s,box-shadow .18s; }
.billing-help-button:hover { background:#fff; border-color:#1677ff; color:#0758cb; box-shadow:0 3px 12px rgb(22 119 255 / .16); }
.billing-help-button:active { background:#f0f6ff; }
.plaza-description { margin-top:.75rem; }
.plaza-notice { margin:0 0 1rem; font-size:.74rem; color:var(--plaza-muted); }
.result-count { flex-shrink:0; min-width:1.5rem; padding:.15rem .4rem; border-radius:.35rem; background:var(--plaza-selected); font-size:.68rem; font-weight:500; font-variant-numeric:tabular-nums; }
.view-switch { display:flex; flex-shrink:0; height:2.5rem; align-items:stretch; padding:.2rem; gap:.2rem; background:var(--plaza-button); border:1px solid var(--plaza-glass-edge); border-radius:.75rem; }
.view-switch button { display:flex; align-items:center; justify-content:center; gap:.35rem; padding:0 .65rem; border-radius:.5rem; color:var(--plaza-ink); font-size:.75rem; font-weight:500; white-space:nowrap; cursor:pointer; transition:background .18s,color .18s,box-shadow .18s; }
.view-switch button:hover { background:var(--plaza-selected); color:var(--plaza-blue); }
.view-switch button.active { background:#1677ff; color:#fff; font-weight:600; box-shadow:0 2px 7px rgb(22 119 255 / .18); }
.plaza-card-grid { display:grid; grid-template-columns:repeat(auto-fit,minmax(min(100%,21rem),1fr)); gap:1.15rem; align-items:stretch; }
.plaza-card-grid.list-view { grid-template-columns:1fr; gap:.7rem; }
.plaza-empty { min-height:20rem; display:flex; flex-direction:column; align-items:center; justify-content:center; gap:1rem; color:var(--plaza-muted); font-size:.85rem; }
.plaza-empty button { padding:.6rem 1rem; border:1px solid var(--plaza-line); background:var(--plaza-button); border-radius:.6rem; color:var(--plaza-blue); }
.plaza-pagination { margin-top:1.2rem; }
.model-skeleton { height:17rem; padding:1.25rem; border:1px solid var(--plaza-glass-edge); border-radius:1.2rem; background:var(--plaza-glass); }
.model-skeleton>div { width:70%; height:1.5rem; margin-bottom:2.1rem; border-radius:.5rem; background:var(--plaza-selected); animation:plaza-pulse 1.5s ease-in-out infinite; }
.model-skeleton>div:nth-child(2) { width:90%; height:4rem; } .model-skeleton>div:nth-child(3) { height:.9rem; width:100%; }
button:focus-visible,select:focus-visible { outline:2px solid #1677ff; outline-offset:3px; }
@keyframes plaza-pulse { 50% { opacity:.4; } }
:global(.dark .plaza-gallery) { --plaza-ink:#dce7f6; --plaza-muted:#9cacc2; --plaza-blue:#72aaff; --plaza-line:#33445a; --plaza-rail:#172435; --plaza-selected:#263449; --plaza-button:rgb(33 49 69 / .8); --plaza-glass:linear-gradient(135deg,rgb(37 56 78 / .86),rgb(29 45 63 / .7)); --plaza-glass-edge:rgb(138 169 200 / .18); }
:global(.dark .plaza-workspace) { background:radial-gradient(ellipse at 40% 30%,rgb(40 86 142 / .25),transparent 70%),#142235; }
@media (min-width:1750px) { .plaza-card-grid:not(.list-view) { grid-template-columns:repeat(3,minmax(0,1fr)); } }
@media (max-width:1100px) { .plaza-workspace { grid-template-columns:18rem minmax(0,1fr); } .plaza-filters,.plaza-results { padding:1rem; } }
@media (min-width:768px) and (max-width:1199px) { .plaza-toolbar { flex-wrap:wrap; } .plaza-search { flex-basis:calc(100% - 13rem); } }
@media (max-width:767px) { .plaza-workspace { grid-template-columns:minmax(0,1fr); } .filters-closed { display:none; } .mobile-chevron { display:block; } .filters-heading { padding-bottom:0; border:0; } .filters-toggle { display:flex; font-size:1rem; } .desktop-filter-title { display:none; } .plaza-filters { border-bottom:1px solid var(--plaza-line); } .plaza-toolbar { flex-wrap:wrap; } .plaza-search { flex-basis:100%; } .view-switch { margin-left:auto; } .price-switch { flex:1; } .price-switch button { flex:1; } .plaza-results { padding:.85rem; } }
@media (prefers-reduced-motion:reduce) { .model-skeleton>div { animation:none; } .billing-help-button,.view-switch button { transition:none; } }
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
