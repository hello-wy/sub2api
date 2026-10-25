<template>
  <BaseDialog :show="!!active" :title="t('modelPlaza.gallery.details')" width="extra-wide" close-on-click-outside @close="$emit('close')">
    <div v-if="active" class="plaza-model-details">
      <header class="detail-heading">
        <span class="detail-icon" :style="{ color: platformAccentColor(active.model.platform) }"><PlatformIcon colored :platform="active.model.platform" size="lg" /></span>
        <div><span>{{ platformLabel(active.model.platform) }}</span><h2>{{ active.model.name }}</h2></div>
        <button type="button" class="detail-copy" @click="copyToClipboard(active.model.name)"><Icon :name="copied ? 'check' : 'copy'" size="sm" />{{ t('modelPlaza.gallery.copy') }}</button>
      </header>
      <p class="detail-intro">{{ t('modelPlaza.gallery.detailIntro') }}</p>
      <section class="group-comparison">
        <h3>{{ t('modelPlaza.gallery.compareGroups') }} <span>{{ variants.length }}</span></h3>
        <div class="comparison-scroll" tabindex="0" :aria-label="t('modelPlaza.gallery.compareGroups')">
          <table>
            <thead><tr><th scope="col">{{ t('modelPlaza.gallery.groups') }}</th><th scope="col">{{ t('modelPlaza.gallery.billing') }}</th><th scope="col">{{ t('modelPlaza.table.input') }} / {{ t('modelPlaza.gallery.requestPrice') }}</th><th scope="col">{{ t('modelPlaza.table.output') }}</th><th scope="col">{{ t('modelPlaza.gallery.channelStatus') }}</th></tr></thead>
            <tbody><tr v-for="variant in variants" :key="variant.key" :class="{ selected: variant.key === active.key }">
              <td><button type="button" class="detail-group-button" :aria-pressed="variant.key === active.key" @click="selected = variant.key"><i :class="{ checked: variant.key === active.key }" />{{ variant.group.name }}<small>{{ plazaRate(variant.group, variant.model) }}×</small></button></td>
              <td>{{ t('modelPlaza.gallery.billingModes.' + (variant.model.pricing?.billing_mode ?? 'token')) }}</td>
              <td>{{ amount(firstPrice(variant).request ?? firstPrice(variant).input) }} <small>{{ priceUnit(variant) }}</small></td>
              <td>{{ amount(firstPrice(variant).output) }}</td>
              <td><span class="detail-health" :class="variant.status.health">{{ t('modelPlaza.gallery.health.' + variant.status.health) }}</span></td>
            </tr></tbody>
          </table>
        </div>
      </section>
      <section class="detail-pricing">
        <div class="detail-section-heading"><h3>{{ active.group.name }}</h3><span>{{ t('modelPlaza.gallery.' + priceMode) }} · {{ unit }}</span></div>
        <p v-if="active.group.description" class="detail-description">{{ active.group.description }}</p>
        <div class="comparison-scroll" tabindex="0" :aria-label="t('modelPlaza.gallery.details')">
          <table class="detail-price-table">
            <thead><tr><th scope="col">{{ t('modelPlaza.gallery.priceTier') }}</th><template v-if="!requestPricing"><th scope="col">{{ t('modelPlaza.table.input') }}</th><th scope="col">{{ t('modelPlaza.table.output') }}</th><th scope="col">{{ t('modelPlaza.gallery.cacheRead') }}</th><th scope="col">{{ t('modelPlaza.gallery.cacheWrite') }}</th><th scope="col">{{ t('modelPlaza.gallery.cacheWrite1h') }}</th></template><th v-else>{{ t('modelPlaza.gallery.requestPrice') }}</th></tr></thead>
            <tbody><tr v-for="(row, index) in prices" :key="index"><td>{{ row.label || t('modelPlaza.gallery.baseTier') }}</td><template v-if="!requestPricing"><td>{{ amount(row.input) }}</td><td>{{ amount(row.output) }}</td><td>{{ amount(row.cacheRead) }}</td><td>{{ amount(row.cacheWrite) }}</td><td>{{ amount(row.cacheWrite1h) }}</td></template><td v-else>{{ amount(row.request) }}</td></tr></tbody>
          </table>
        </div>
        <div class="detail-notes">
          <p v-if="active.model.long_context_basis === 'marginal'">{{ t('modelPlaza.table.tierHintMarginal') }}</p>
          <template v-if="priceMode === 'paid'">
            <p v-if="active.model.time_pricing?.periods.length">{{ t('modelPlaza.gallery.timePricing') }} · {{ active.model.time_pricing.timezone }}<span v-if="active.model.time_pricing.weekdays_only"> · {{ t('modelPlaza.table.timePricingWeekdays') }}</span><span v-for="period in active.model.time_pricing.periods" :key="period.start_time" class="detail-period">{{ period.start_time }}–{{ period.end_time }} · {{ period.multiplier }}×</span></p>
            <p v-if="active.group.peak_rate_enabled">{{ t('modelPlaza.gallery.peakHint', { window: active.group.peak_start + '–' + active.group.peak_end }) }} {{ active.group.peak_rate_multiplier }}×</p>
            <p v-for="(multiplier, effort) in active.model.pricing?.reasoning_effort_multipliers ?? {}" :key="effort">{{ t('modelPlaza.table.reasoningMultiplierHint', { effort, multiplier }) }}</p>
          </template>
          <p>{{ t('modelPlaza.gallery.priceHint') }}</p>
        </div>
      </section>
      <section class="detail-monitor">
        <h3>{{ t('modelPlaza.gallery.channelStatus') }} <span>{{ active.group.name }}</span></h3>
        <PlazaStatusFooter :status="active.status" :show-throughput="showThroughput" />
        <p>{{ t('modelPlaza.gallery.monitorExplanation') }}</p>
      </section>
    </div>
  </BaseDialog>
</template>
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import Icon from '@/components/icons/Icon.vue'
import PlazaStatusFooter from './PlazaStatusFooter.vue'
import { useClipboard } from '@/composables/useClipboard'
import { platformAccentColor, platformLabel } from '@/utils/platformColors'
import { formatScaled } from '@/utils/pricing'
import { plazaPrices, plazaRate, type PlazaModelVariant, type PlazaPriceMode } from '@/utils/model-plaza'
const props = defineProps<{ variants: PlazaModelVariant[]; selectedKey: string; priceMode: PlazaPriceMode; showThroughput: boolean }>()
defineEmits<{ close: [] }>()
const { t } = useI18n()
const { copied, copyToClipboard } = useClipboard()
const selected = ref('')
watch([() => props.selectedKey, () => props.variants.length > 0], ([key]) => { selected.value = key }, { immediate: true })
const active = computed(() => props.variants.find(variant => variant.key === selected.value) ?? props.variants[0])
const prices = computed(() => active.value ? plazaPrices(active.value.model, active.value.group, props.priceMode) : [])
const requestPricing = computed(() => props.priceMode === 'paid' && !!active.value?.model.pricing && active.value.model.pricing.billing_mode !== 'token')
const unit = computed(() => t(requestPricing.value ? (active.value?.model.pricing?.billing_mode === 'image' ? 'modelPlaza.table.perUnitImage' : 'modelPlaza.table.perUnitRequest') : 'modelPlaza.gallery.tokenUnit'))
const firstPrice = (variant: PlazaModelVariant) => plazaPrices(variant.model, variant.group, props.priceMode)[0]
const priceUnit = (variant: PlazaModelVariant) => t(props.priceMode === 'paid' && variant.model.pricing && variant.model.pricing.billing_mode !== 'token' ? (variant.model.pricing.billing_mode === 'image' ? 'modelPlaza.table.perUnitImage' : 'modelPlaza.table.perUnitRequest') : 'modelPlaza.gallery.tokenUnit')
const amount = (value: number | null) => formatScaled(value, 1, 2)
</script>
<style scoped>
.detail-heading { display:flex; align-items:center; gap:1rem; }
.detail-icon { display:grid; place-items:center; flex-shrink:0; width:3.25rem; height:3.25rem; border:1px solid var(--plaza-line); border-radius:.9rem; background:var(--plaza-surface); }
.detail-heading>div { min-width:0; }
.detail-heading>div>span { font-size:.75rem; color:var(--plaza-muted); }
.detail-heading h2 { font-size:1.35rem; font-weight:650; overflow-wrap:anywhere; letter-spacing:-.03em; }
.detail-copy { display:flex; align-items:center; gap:.4rem; flex-shrink:0; margin-left:auto; padding:.5rem .7rem; border:1px solid var(--plaza-line); border-radius:.5rem; color:var(--plaza-muted); font-size:.7rem; }
.detail-intro { margin:1rem 0 1.5rem; color:var(--plaza-muted); font-size:.75rem; line-height:1.8; }
h3 { display:flex; align-items:center; gap:.6rem; font-size:.8rem; font-weight:600; }
h3>span { color:var(--plaza-muted); font-size:.7rem; font-weight:400; }
.comparison-scroll { overflow-x:auto; border:1px solid var(--plaza-line); border-radius:.65rem; margin-top:.8rem; }
table { width:100%; border-collapse:collapse; text-align:left; font-size:.8rem; white-space:nowrap; }
th { padding:.75rem .85rem; background:var(--plaza-subtle); color:var(--plaza-muted); font-size:.72rem; font-weight:500; }
td { padding:.75rem .85rem; border-top:1px solid var(--plaza-line); font-variant-numeric:tabular-nums; }
td small { font-size:.68rem; color:var(--plaza-muted); }
tr.selected td { background:var(--plaza-selected); }
.detail-group-button { display:flex; align-items:center; gap:.55rem; text-align:left; }
.detail-group-button>i { width:.7rem; height:.7rem; border:1px solid var(--plaza-muted); border-radius:50%; }
.detail-group-button>i.checked { border:3px solid var(--plaza-blue); }
.detail-health { color:var(--plaza-muted); font-size:.72rem; }
.detail-health.healthy { color:var(--plaza-green); } .detail-health.warning { color:var(--plaza-amber); } .detail-health.critical { color:var(--plaza-red); }
.detail-pricing,.detail-monitor { margin-top:1.6rem; }
.detail-section-heading { display:flex; flex-wrap:wrap; align-items:center; justify-content:space-between; gap:.6rem; }
.detail-section-heading>span { font-size:.7rem; color:var(--plaza-muted); }
.detail-description { color:var(--plaza-muted); font-size:.75rem; margin:.5rem 0; }
.detail-price-table td:not(:first-child) { font-family:var(--plaza-numbers); }
.detail-notes { margin-top:1rem; font-size:.7rem; line-height:1.9; color:var(--plaza-muted); }
.detail-notes p+p { margin-top:.4rem; }
.detail-period { display:inline-block; margin-left:.5rem; padding:.1rem .4rem; background:var(--plaza-subtle); border-radius:.3rem; }
.detail-monitor>p { margin-top:.8rem; padding:.75rem; background:var(--plaza-subtle); border-radius:.5rem; color:var(--plaza-muted); font-size:.7rem; line-height:1.8; }
button:focus-visible,.comparison-scroll:focus-visible { outline:2px solid var(--plaza-blue); outline-offset:3px; }
@media (max-width:640px) { .detail-copy { padding:.5rem; font-size:0; gap:0; } .detail-heading h2 { font-size:1.1rem; } }
</style>
