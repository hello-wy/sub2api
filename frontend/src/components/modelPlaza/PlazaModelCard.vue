<template>
  <article class="plaza-model-card" :class="{ 'model-list-row': view === 'list' }" :data-model="activeModel.name" @click="handleCardClick">
    <header class="model-heading">
      <span class="provider-mark"><PlatformIcon colored :platform="activeModel.platform" size="lg" /></span>
      <div class="model-identity">
        <h3><button type="button" aria-haspopup="dialog" :title="activeModel.name" @click="$emit('details', activeKey)">{{ activeModel.name }}</button></h3>
        <span v-if="priceMode === 'paid'" class="rate-tag" :title="independentImage ? t('modelPlaza.gallery.independentImage') : t('modelPlaza.gallery.rateHint')">{{ t('modelPlaza.filters.rateLabel') }} {{ rate }}×</span>
      </div>
      <div class="model-actions">
        <div class="model-group">
          <label v-if="options.length > 1" class="card-group-picker">
            <span class="group-coverage sr-only">{{ t('modelPlaza.gallery.groupCoverage', { count: options.length }) }}</span>
            <select v-model="groupSelection" :aria-label="t('modelPlaza.gallery.selectGroup') + ' · ' + activeModel.name" :title="activeGroup.name">
              <option v-for="option in options" :key="option.key" :value="option.key">{{ option.group.name }}</option>
            </select>
          </label>
          <span v-else class="single-group" :title="activeGroup.description || activeGroup.name">{{ activeGroup.name }}</span>
        </div>
        <button type="button" class="copy-model" :title="t('modelPlaza.gallery.copy')" :aria-label="t('modelPlaza.gallery.copy') + ' ' + activeModel.name" @click="copyToClipboard(activeModel.name)"><Icon :name="copied ? 'check' : 'copy'" size="sm" /></button>
      </div>
      <div class="model-prices" :data-price-mode="priceMode" :aria-label="t('modelPlaza.gallery.' + priceMode)">
        <div v-if="requestPricing" class="price-pair request-price">
          <div><span>{{ t('modelPlaza.gallery.requestPrice') }}</span><strong>{{ requestRange }}<small>{{ unit }}</small></strong></div>
        </div>
        <div v-else class="price-pair">
          <div><span>{{ t('modelPlaza.table.input') }}</span><strong>{{ price(prices[0].input) }}<small v-if="prices[0].input != null">{{ unit }}</small></strong></div>
          <div><span>{{ t('modelPlaza.table.output') }}</span><strong>{{ price(prices[0].output) }}<small v-if="prices[0].output != null">{{ unit }}</small></strong></div>
        </div>
        <div v-if="!requestPricing" class="cache-prices">
          <div><span>{{ t('modelPlaza.gallery.cacheRead') }}</span><b>{{ price(prices[0].cacheRead) }}<small v-if="prices[0].cacheRead != null">{{ unit }}</small></b></div>
          <div><span>{{ t('modelPlaza.gallery.cacheWrite') }}</span><b>{{ price(prices[0].cacheWrite) }}<small v-if="prices[0].cacheWrite != null">{{ unit }}</small></b></div>
        </div>
        <p v-if="independentImage && priceMode === 'paid'" class="price-note">{{ t('modelPlaza.gallery.independentImage') }}</p>
      </div>
    </header>
    <div class="model-tags">
      <span class="billing-tag">{{ requestPricing ? t(activeModel.pricing?.billing_mode === 'image' ? 'modelPlaza.table.perImage' : 'modelPlaza.table.perRequest') : t('modelPlaza.gallery.usage') }}</span>
      <span v-if="prices.length > 1" class="tier-tag"><Icon name="trendingUp" size="xs" />{{ t('modelPlaza.gallery.tiered') }}</span>
      <span v-if="priceMode === 'paid' && activeModel.time_pricing?.periods.length" class="time-tag">{{ t('modelPlaza.gallery.timePricing') }}</span>
      <span v-if="activeGroup.is_exclusive" class="neutral-tag">{{ t('modelPlaza.badges.exclusive') }}</span>
      <span v-if="activeGroup.subscription_type === 'subscription'" class="neutral-tag">{{ t('modelPlaza.badges.subscription') }}</span>
      <span class="model-health sr-only" :class="activeStatus.health">{{ t('modelPlaza.gallery.selectedGroupStatus', { group: activeGroup.name }) }} · {{ t('modelPlaza.gallery.health.' + activeStatus.health) }}</span>
      <span class="status-coverage sr-only" :title="t('modelPlaza.gallery.statusCoverageHint')">{{ t('modelPlaza.gallery.monitorCoverage', { count: statusCount, total: options.length }) }}</span>
    </div>
    <PlazaStatusFooter :status="activeStatus" :show-throughput="showThroughput" />
  </article>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import Icon from '@/components/icons/Icon.vue'
import PlazaStatusFooter from './PlazaStatusFooter.vue'
import { useClipboard } from '@/composables/useClipboard'
import { formatScaled } from '@/utils/pricing'
import { plazaPrices, plazaRate, plazaHasStatusData, type PlazaPriceMode, type PlazaStatus, type PlazaModelVariant } from '@/utils/model-plaza'
import type { PlazaModel, ModelPlazaGroup } from '@/api/modelPlaza'

const props = defineProps<{ model: PlazaModel; group: ModelPlazaGroup; variants?: PlazaModelVariant[]; priceMode: PlazaPriceMode; status: PlazaStatus; showThroughput: boolean; view?: 'grid' | 'list' }>()
const emit = defineEmits<{ details: [key: string] }>()
const { t } = useI18n()
function handleCardClick(event: MouseEvent) {
  const target = event.target
  if (target instanceof Element && target.closest('button, select, input, a, label')) return
  emit('details', activeKey.value)
}
const { copied, copyToClipboard } = useClipboard()
const selectedKey = ref('')
const options = computed(() => props.variants?.length ? props.variants : [{ key: String(props.group.id), model: props.model, group: props.group, status: props.status }])
const active = computed(() => options.value.find(option => option.key === selectedKey.value) ?? options.value[0])
const activeKey = computed(() => active.value.key)
const groupSelection = computed({ get: () => activeKey.value, set: (key: string) => { selectedKey.value = key } })
const activeModel = computed(() => active.value.model)
const activeGroup = computed(() => active.value.group)
const activeStatus = computed(() => active.value.status)
const statusCount = computed(() => options.value.filter(option => plazaHasStatusData(option.status)).length)
const price = (n: number | null) => formatScaled(n, 1, 4)
const rate = computed(() => plazaRate(activeGroup.value, activeModel.value))
const prices = computed(() => plazaPrices(activeModel.value, activeGroup.value, props.priceMode))
const requestPricing = computed(() => props.priceMode === 'paid' && !!activeModel.value.pricing && activeModel.value.pricing.billing_mode !== 'token')
const independentImage = computed(() => activeModel.value.pricing?.billing_mode === 'image' && activeGroup.value.image_rate_independent)
const unit = computed(() => requestPricing.value ? t(activeModel.value.pricing?.billing_mode === 'image' ? 'modelPlaza.table.perUnitImage' : 'modelPlaza.table.perUnitRequest') : '/M')
const requestRange = computed(() => {
  const values = prices.value.map(row => row.request).filter((v): v is number => v != null)
  if (!values.length) return '—'
  const min = Math.min(...values), max = Math.max(...values)
  return min === max ? price(min) : price(min) + ' – ' + price(max)
})
</script>

<style scoped>
.plaza-model-card { min-width:0; display:flex; flex-direction:column; min-height:13rem; padding:.65rem; border:1px solid var(--plaza-line); border-radius:.75rem; background:var(--plaza-surface); container-type:inline-size; transition:border-color .18s; cursor:pointer; }
.plaza-model-card:focus-within { outline:2px solid var(--plaza-blue); outline-offset:3px; }
.plaza-model-card:hover { border-color:var(--plaza-hover-line); }
.model-heading { display:grid; grid-template-columns:3rem minmax(0,1fr) auto; align-items:start; gap:.15rem .75rem; }
.provider-mark { grid-row:1 / 3; display:grid; place-items:center; width:3rem; height:3rem; border-radius:.8rem; background:var(--plaza-subtle); }
.provider-mark :deep(svg) { width:2rem; height:2rem; }
.model-identity { display:flex; align-items:center; flex-wrap:wrap; gap:.3rem .5rem; min-width:0; min-height:1.6rem; }
h3 { min-width:0; margin:0; font-size:1rem; font-weight:650; letter-spacing:-.015em; line-height:1.5; overflow-wrap:anywhere; }
h3 button { text-align:left; color:var(--plaza-ink); }
h3 button:hover { color:var(--plaza-blue); }
.rate-tag { flex-shrink:0; border-radius:.2rem; padding:.05rem .3rem; font-size:.65rem; line-height:1.3; color:var(--plaza-muted); background:var(--plaza-subtle); }
.model-actions { display:flex; align-items:center; gap:.35rem; padding-top:.1rem; min-width:0; }
.model-group { min-width:0; max-width:8rem; color:var(--plaza-blue); font-size:.7rem; line-height:1.4; }
.card-group-picker { display:block; min-width:0; }
.card-group-picker select { display:block; max-width:8rem; width:100%; border:0; border-radius:.2rem; padding:.1rem 1.15rem .1rem .3rem; background-color:var(--plaza-selected); color:var(--plaza-blue); font-size:.7rem; text-overflow:ellipsis; }
.single-group { display:block; min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; padding:.05rem .3rem; border-radius:.2rem; background:var(--plaza-selected); font-family:Georgia,'Times New Roman',serif; font-style:italic; }
.copy-model { display:grid; place-items:center; width:1.4rem; height:1.4rem; border-radius:.25rem; color:var(--plaza-muted); flex-shrink:0; }
.copy-model:hover { color:var(--plaza-blue); background:var(--plaza-selected); }
.model-prices { grid-column:2 / 4; color:var(--plaza-muted); font-size:.75rem; line-height:1.5; }
.price-pair,.cache-prices { display:flex; flex-direction:column; }
.price-pair>div,.cache-prices>div { display:flex; flex-wrap:wrap; align-items:baseline; gap:.2rem; }
.model-prices strong,.model-prices b { color:var(--plaza-ink); font-family:Georgia,'Times New Roman',serif; font-weight:400; font-style:italic; font-size:.875rem; font-variant-numeric:tabular-nums; overflow-wrap:anywhere; }
.model-prices small { font-size:inherit; color:var(--plaza-muted); }
.price-note { margin-top:.25rem; font-size:.7rem; }
.model-tags { display:flex; flex-wrap:wrap; align-items:center; gap:.45rem; margin-top:auto; padding-top:1.5rem; padding-bottom:.65rem; }
.model-tags>span:not(.sr-only) { display:inline-flex; align-items:center; gap:.25rem; padding:.15rem .5rem; border-radius:2rem; font-size:.7rem; line-height:1.4; }
.model-tags>.billing-tag,.model-tags>.time-tag { color:#6848ba; background:#eee8fc; }
.model-tags>.tier-tag { color:#813b9c; background:#f2e5f7; }
.model-tags>.neutral-tag { color:var(--plaza-muted); background:var(--plaza-subtle); }
.plaza-model-card :deep(.plaza-status-footer) { margin:0; }
button:focus-visible,select:focus-visible { outline:2px solid var(--plaza-blue); outline-offset:3px; }
:global(.dark .plaza-model-card .model-tags>.billing-tag),:global(.dark .plaza-model-card .model-tags>.time-tag) { color:#c6b3ff; background:#33294d; }
:global(.dark .plaza-model-card .model-tags>.tier-tag) { color:#e0a8ee; background:#422c4c; }
.model-list-row { min-height:12rem; }
@container (max-width:26rem) {
  .model-heading { grid-template-columns:3rem minmax(0,1fr); }
  .model-identity { grid-column:2; }
  .model-actions { grid-column:2; grid-row:2; flex-wrap:nowrap; padding:.1rem 0; }
  .provider-mark { grid-row:1 / 4; }
  .model-prices { grid-column:2; grid-row:3; }
  .model-group { flex:0 1 auto; max-width:12rem; }
  .card-group-picker select { max-width:100%; }
}
@media (prefers-reduced-motion:reduce) { .plaza-model-card { transition:none; } }
</style>
