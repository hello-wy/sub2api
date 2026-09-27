<template>
  <article class="plaza-model-card" :class="{ 'model-list-row': view === 'list' }" :data-model="model.name">
    <header class="model-heading">
      <span class="provider-mark" :style="{ color: platformAccentColor(model.platform) }">
        <PlatformIcon :platform="model.platform" size="lg" />
      </span>
      <div class="model-identity">
        <h3 :title="model.name">{{ model.name }}</h3>
        <div class="model-group">
          <span :title="group.description || group.name">{{ group.name }}</span>
          <b v-if="priceMode === 'paid'" :title="independentImage ? t('modelPlaza.gallery.independentImage') : t('modelPlaza.table.rate')">{{ rate }}×</b>
        </div>
      </div>
      <span class="model-health" :class="status.health">{{ t('modelPlaza.gallery.health.' + status.health) }}</span>
    </header>

    <div class="model-prices" :data-price-mode="priceMode">
      <span class="sr-only">{{ unit }} <template v-if="prices[0].label">· {{ prices[0].label }}</template></span>
      <div v-if="requestPricing" class="price-pair request-price">
        <div><span>{{ t('modelPlaza.gallery.requestPrice') }}</span><strong>{{ requestRange }} <small class="price-unit">{{ unit }}</small></strong></div>
      </div>
      <div v-else class="price-pair">
        <div><span>{{ t('modelPlaza.table.input') }}</span><strong>{{ price(prices[0].input) }} <small v-if="prices[0].input != null" class="price-unit">{{ unit }}</small></strong></div>
        <div><span>{{ t('modelPlaza.table.output') }}</span><strong>{{ price(prices[0].output) }} <small v-if="prices[0].output != null" class="price-unit">{{ unit }}</small></strong></div>
      </div>
      <div v-if="!requestPricing" class="cache-prices">
        <span>{{ t('modelPlaza.gallery.cacheRead') }} <b>{{ price(prices[0].cacheRead) }}</b><small v-if="prices[0].cacheRead != null" class="cache-unit">{{ t('modelPlaza.gallery.cacheUnit') }}</small></span>
        <span>{{ t('modelPlaza.gallery.cacheWrite') }} <b>{{ price(prices[0].cacheWrite) }}</b><small v-if="prices[0].cacheWrite != null" class="cache-unit">{{ t('modelPlaza.gallery.cacheUnit') }}</small></span>
      </div>
      <p v-if="independentImage && priceMode === 'paid'" class="price-note">{{ t('modelPlaza.gallery.independentImage') }}</p>
    </div>

    <div class="model-tags">
      <span class="billing-tag">{{ requestPricing ? t(model.pricing?.billing_mode === 'image' ? 'modelPlaza.table.perImage' : 'modelPlaza.table.perRequest') : t('modelPlaza.gallery.usage') }}</span>
      <span v-if="prices.length > 1" class="tier-tag"><svg viewBox="0 0 16 16" aria-hidden="true"><path d="M1 13h5V8h5V3h4" /></svg>{{ t('modelPlaza.gallery.tiered') }}</span>
      <span v-if="priceMode === 'paid' && model.time_pricing?.periods.length" class="time-tag">{{ t('modelPlaza.gallery.timePricing') }}</span>
      <span v-if="group.is_exclusive" class="neutral-tag">{{ t('modelPlaza.badges.exclusive') }}</span>
      <span v-if="group.subscription_type === 'subscription'" class="neutral-tag">{{ t('modelPlaza.badges.subscription') }}</span>
      <button type="button" class="copy-model" :title="t('modelPlaza.gallery.copy')" :aria-label="t('modelPlaza.gallery.copy') + ' ' + model.name" @click="copyToClipboard(model.name)"><Icon :name="copied ? 'check' : 'copy'" size="xs" /></button>
    </div>

    <PlazaStatusFooter :status="status" :show-throughput="showThroughput" />
  </article>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import Icon from '@/components/icons/Icon.vue'
import PlazaStatusFooter from './PlazaStatusFooter.vue'
import { useClipboard } from '@/composables/useClipboard'
import { platformAccentColor } from '@/utils/platformColors'
import { formatScaled } from '@/utils/pricing'
import { plazaPrices, plazaRate, type PlazaPriceMode, type PlazaStatus } from '@/utils/model-plaza'
import type { PlazaModel, ModelPlazaGroup } from '@/api/modelPlaza'
const props = defineProps<{ model: PlazaModel; group: ModelPlazaGroup; priceMode: PlazaPriceMode; status: PlazaStatus; showThroughput: boolean; view?: 'grid' | 'list' }>()
const { t } = useI18n()
const { copied, copyToClipboard } = useClipboard()
const price = (n: number | null) => formatScaled(n, 1, 2)
const rate = computed(() => plazaRate(props.group, props.model))
const prices = computed(() => plazaPrices(props.model, props.group, props.priceMode))
const requestPricing = computed(() => props.priceMode === 'paid' && !!props.model.pricing && props.model.pricing.billing_mode !== 'token')
const independentImage = computed(() => props.model.pricing?.billing_mode === 'image' && props.group.image_rate_independent)
const unit = computed(() => requestPricing.value ? t(props.model.pricing?.billing_mode === 'image' ? 'modelPlaza.table.perUnitImage' : 'modelPlaza.table.perUnitRequest') : t('modelPlaza.gallery.tokenUnit'))
const requestRange = computed(() => {
  const values = prices.value.map(row => row.request).filter((v): v is number => v != null)
  if (!values.length) return '—'
  const min = Math.min(...values), max = Math.max(...values)
  return min === max ? price(min) : price(min) + ' – ' + price(max)
})
</script>
<style scoped>
.plaza-model-card { min-width:0; min-height:16rem; display:flex; flex-direction:column; padding:1.15rem; border:1px solid var(--plaza-glass-edge); border-radius:1.2rem; background:var(--plaza-glass); box-shadow:0 12px 32px rgb(55 99 152 / .08),inset 0 1px 0 rgb(255 255 255 / .3); backdrop-filter:blur(20px); transition:box-shadow .2s,border-color .2s; }
.plaza-model-card:hover { border-color:#a9cafa; box-shadow:0 15px 35px rgb(55 99 152 / .13); }
.model-heading { display:flex; align-items:flex-start; gap:.7rem; }
.provider-mark { display:grid; place-items:center; width:2.3rem; height:2.3rem; flex-shrink:0; border-radius:.7rem; background:var(--plaza-button); }
.model-identity { min-width:0; flex:1; } h3 { margin:.1rem 0 .4rem; font-size:.9375rem; font-weight:600; color:var(--plaza-ink); overflow-wrap:anywhere; letter-spacing:-.025em; }
.model-group { display:flex; gap:.55rem; align-items:center; font-size:.65rem; }
.model-group>span { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; border-radius:.25rem; background:var(--plaza-selected); color:var(--plaza-muted); padding:.15rem .5rem; }
.model-group>b { font-weight:550; color:var(--plaza-blue); white-space:nowrap; }
.model-health { flex-shrink:0; padding:.28rem .5rem; border-radius:.4rem; font-size:.62rem; font-weight:600; background:var(--plaza-selected); color:var(--plaza-muted); }
.model-health.healthy { color:#008965; background:rgb(5 185 133 / .08); } .model-health.warning { color:#a76900; background:rgb(247 174 29 / .12); } .model-health.critical { color:#da3755; background:rgb(254 86 112 / .09); }
.model-prices { padding:1.1rem 0 .7rem; }
.price-unit { white-space:nowrap; font-size:.7rem; font-weight:400; letter-spacing:0; color:var(--plaza-muted); }
.price-pair { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:.75rem; }
.price-pair>div { display:flex; flex-direction:column; gap:.15rem; } .price-pair span { color:var(--plaza-muted); font-size:.7rem; }
.price-pair strong { display:flex; align-items:baseline; flex-wrap:wrap; gap:0 .25rem; font-size:clamp(1.2rem,1.6vw,1.65rem); letter-spacing:-.04em; font-weight:650; color:var(--plaza-ink); font-variant-numeric:tabular-nums; overflow-wrap:anywhere; }
.request-price { grid-template-columns:1fr; } .request-price strong { font-size:1.35rem; }
.cache-prices { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:.35rem .75rem; margin-top:.5rem; font-size:.68rem; color:var(--plaza-muted); }
.cache-unit { margin-left:.15rem; font-size:.6rem; white-space:nowrap; }
.cache-prices b { font-weight:450; font-variant-numeric:tabular-nums; } .price-note { color:var(--plaza-muted); font-size:.65rem; margin-top:.5rem; }
.model-tags { display:flex; flex-wrap:wrap; align-items:center; gap:.4rem; margin-bottom:1rem; }
.plaza-model-card :deep(.plaza-status-footer) { margin-top:auto; }
.model-tags>span { padding:.22rem .55rem; font-size:.65rem; border-radius:1rem; display:inline-flex; align-items:center; gap:.3rem; }
.billing-tag { color:#7551b9; background:rgb(183 139 237 / .19); } .tier-tag { color:#a54ead; background:rgb(222 137 219 / .19); }
.tier-tag svg { width:.75rem; height:.75rem; fill:none; stroke:currentColor; stroke-width:2; }
.time-tag,.neutral-tag { color:var(--plaza-muted); background:var(--plaza-selected); }
.copy-model { display:grid; place-items:center; margin-left:auto; width:1.6rem; height:1.6rem; color:var(--plaza-muted); border-radius:.4rem; }
.copy-model:hover { color:var(--plaza-blue); background:var(--plaza-selected); }
button:focus-visible { outline:2px solid #1677ff; outline-offset:3px; }
:global(.dark .plaza-model-card .billing-tag) { color:#d0b7ff; }
:global(.dark .plaza-model-card .tier-tag) { color:#efb3ed; }
:global(.dark .model-health.healthy) { color:#55d9b1; }
:global(.dark .model-health.warning) { color:#f4c063; }
:global(.dark .model-health.critical) { color:#ff8b9e; }
.model-list-row { min-height:0; padding:1rem; border-radius:.85rem; }
.model-list-row .model-prices { padding:.75rem 0; }
.model-list-row .price-pair strong { font-size:1.15rem; }
.model-list-row .model-tags { margin-bottom:.6rem; }
@container plaza-results (min-width:40rem) {
  .model-list-row { display:grid; grid-template-columns:minmax(0,1fr) minmax(0,1.1fr); grid-template-areas:"identity prices" "tags status"; align-items:center; column-gap:1.5rem; row-gap:.7rem; }
  .model-list-row .model-heading { grid-area:identity; }
  .model-list-row .model-prices { grid-area:prices; padding:0; }
  .model-list-row .model-tags { grid-area:tags; margin:0; }
  .model-list-row :deep(.plaza-status-footer) { grid-area:status; margin:0; padding:0; border:0; }
}
@container plaza-results (min-width:62rem) {
  .model-list-row { grid-template-columns:minmax(0,1.05fr) minmax(0,1.15fr) minmax(0,1fr); grid-template-areas:"identity prices status" "tags prices status"; column-gap:1.5rem; }
  .model-list-row :deep(.plaza-status-footer) { align-self:stretch; display:flex; flex-direction:column; align-items:stretch; justify-content:center; border-left:1px solid var(--plaza-line); padding-left:1.25rem; }
  .model-list-row :deep(.status-history) { flex:0; width:100%; }
}
@media (prefers-reduced-motion:reduce) { .plaza-model-card { transition:none; } }
</style>
