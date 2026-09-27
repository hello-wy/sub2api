<template>
  <BaseDialog :show="show" :title="t('modelPlaza.gallery.help')" width="wide" close-on-click-outside @close="$emit('close')">
    <div ref="guide" class="billing-guide">
      <p class="guide-intro">{{ t('modelPlaza.guide.intro') }}</p>
      <div class="guide-pages" role="group" :aria-label="t('modelPlaza.gallery.help')">
        <button v-for="(name, index) in ['pricePage', 'costPage']" :key="name" type="button" :class="{ selected: page === index }" :aria-pressed="page === index" @click="page = index">0{{ index + 1 }} <span>{{ t('modelPlaza.guide.' + name) }}</span></button>
        <button class="guide-replay" type="button" @click="replay">{{ t('modelPlaza.guide.replay') }} <Icon name="refresh" size="xs" /></button>
      </div>
      <div :key="page" class="guide-columns">
        <section class="guide-formula" :aria-label="t('modelPlaza.guide.formula')">
          <h4>{{ t('modelPlaza.guide.formula') }}</h4>
          <button v-for="(item, index) in steps" :key="item.title" type="button" class="formula-step" :class="{ highlighted: activeStep === index }" :aria-pressed="activeStep === index" @click="selectStep(index)">
            <span class="step-heading"><span class="step-number">0{{ index + 1 }}</span><b>{{ item.title }}</b></span>
            <strong>{{ item.formula }}</strong><span class="step-description">{{ item.description }}</span>
          </button>
        </section>
        <section class="guide-example" :aria-label="t('modelPlaza.guide.example')">
          <h4>{{ t('modelPlaza.guide.example') }}</h4>
          <label v-if="entries.length" class="example-picker"><span>{{ t('modelPlaza.guide.modelGroup') }}</span><select v-model="selectedKey"><option v-for="entry in entries" :key="entry.key" :value="entry.key">{{ entry.model.name }} · {{ entry.group.name }}</option></select></label>
          <template v-if="selected">
            <div class="example-heading"><strong>{{ selected.model.name }}</strong><span>{{ selected.group.name }} · {{ rate }}×</span></div>
            <div class="example-calculation">
              <div v-for="(item, index) in exampleSteps" :key="item.label" class="example-step" :class="{ highlighted: activeStep === index, 'example-total': index === 2 }">
                <span class="step-number">0{{ index + 1 }}</span><div><span>{{ item.label }}</span><strong>{{ item.value }}</strong><small v-if="item.detail">{{ item.detail }}</small></div>
              </div>
            </div>
            <p class="example-note">{{ t(page === 0 ? 'modelPlaza.guide.priceExampleNote' : 'modelPlaza.guide.costExampleNote') }}</p>
          </template>
          <p v-else class="example-note">{{ t('modelPlaza.guide.noExample') }}</p>
        </section>
      </div>
      <div class="guide-notes"><p>{{ t('modelPlaza.gallery.priceHint') }}</p><slot /></div>
      <footer class="guide-footer">
        <span class="guide-progress" aria-hidden="true"><i v-for="n in 2" :key="n" :class="{ current: page === n - 1 }" /></span>
        <button v-if="page === 1" type="button" class="guide-back" @click="page = 0">{{ t('modelPlaza.guide.previous') }}</button>
        <button type="button" class="guide-next" @click="page === 0 ? page = 1 : $emit('close')">{{ t(page === 0 ? 'modelPlaza.guide.next' : 'modelPlaza.guide.done') }}<Icon v-if="page === 0" name="chevronRight" size="xs" /></button>
      </footer>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useEventListener, usePreferredReducedMotion } from '@vueuse/core'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { plazaPrices, plazaRate } from '@/utils/model-plaza'
import { formatScaled } from '@/utils/pricing'
import type { ModelPlazaGroup, PlazaModel } from '@/api/modelPlaza'

const props = defineProps<{ show: boolean; entries: Array<{ key: string; model: PlazaModel; group: ModelPlazaGroup }> }>()
defineEmits<{ close: [] }>()
const { t } = useI18n()
const guide = ref<HTMLElement | null>(null)
const page = ref(0), activeStep = ref(0), selectedKey = ref('')
const reducedMotion = usePreferredReducedMotion()
let timer: ReturnType<typeof setInterval> | undefined
const selected = computed(() => props.entries.find(e => e.key === selectedKey.value) ?? props.entries[0])
watch(selected, entry => { selectedKey.value = entry?.key ?? '' }, { immediate: true })
const requestPricing = computed(() => !!selected.value?.model.pricing && selected.value.model.pricing.billing_mode !== 'token')
const rate = computed(() => selected.value ? plazaRate(selected.value.group, selected.value.model) : 1)
const paid = computed(() => selected.value ? plazaPrices(selected.value.model, selected.value.group, 'paid')[0] : null)
const base = computed(() => selected.value ? plazaPrices(selected.value.model, { ...selected.value.group, rate_multiplier: 1, user_rate_multiplier: 1, image_rate_multiplier: 1 }, 'paid')[0] : null)
const amount = (value: number | null | undefined) => formatScaled(value ?? null, 1, 2)
const unit = computed(() => t(requestPricing.value ? (selected.value?.model.pricing?.billing_mode === 'image' ? 'modelPlaza.table.perUnitImage' : 'modelPlaza.table.perUnitRequest') : 'modelPlaza.guide.perMillion'))
const cost = computed(() => {
  if (!paid.value) return null
  if (requestPricing.value) return paid.value.request == null ? null : paid.value.request * 10
  return paid.value.input == null || paid.value.output == null ? null : paid.value.input * .01 + paid.value.output * .002
})
const steps = computed(() => [0, 1, 2].map(index => {
  const prefix = `modelPlaza.guide.${page.value === 0 ? 'price' : 'cost'}${index + 1}`
  return { title: t(prefix + 'Title'), formula: t(prefix + 'Formula'), description: t(prefix + 'Description') }
}))
const exampleSteps = computed(() => {
  const label = (key: string) => t('modelPlaza.guide.' + key)
  if (page.value === 0) {
    const raw = requestPricing.value ? base.value?.request : base.value?.input
    const price = requestPricing.value ? paid.value?.request : paid.value?.input
    return [
      { label: label(requestPricing.value ? 'baseRequest' : 'baseInput'), value: `${amount(raw)} ${unit.value}`, detail: label('baseSource') },
      { label: label('effectiveRate'), value: `× ${rate.value}`, detail: selected.value?.group.name },
      { label: label('paidUnit'), value: `${amount(price)} ${unit.value}`, detail: label('alreadyMultiplied') },
    ]
  }
  return [
    { label: label('exampleUsage'), value: label(requestPricing.value ? 'requestUsage' : 'tokenUsage'), detail: label('usageOnlyExample') },
    { label: label('lineCosts'), value: requestPricing.value ? `10 × ${amount(paid.value?.request)}` : `${amount(paid.value?.input == null ? null : paid.value.input * .01)} + ${amount(paid.value?.output == null ? null : paid.value.output * .002)}`, detail: label(requestPricing.value ? 'quantityTimesPrice' : 'inputPlusOutput') },
    { label: label('exampleTotal'), value: amount(cost.value), detail: label('usd') },
  ]
})
function stop() { if (timer) clearInterval(timer); timer = undefined }
function replay() {
  stop()
  activeStep.value = reducedMotion.value === 'reduce' ? 2 : 0
  if (!props.show || reducedMotion.value === 'reduce') return
  timer = setInterval(() => { activeStep.value += 1; if (activeStep.value >= 2) stop() }, 2000)
}
function selectStep(index: number) { stop(); activeStep.value = index }
watch(() => props.show, show => { if (show) { page.value = 0; replay() } else stop() }, { immediate: true })
watch([page, selectedKey, reducedMotion], () => { if (props.show) replay() })
onBeforeUnmount(stop)
// Keep keyboard navigation inside this guide, including the shared dialog's close button.
useEventListener(document, 'keydown', (event: KeyboardEvent) => {
  if (!props.show || event.key !== 'Tab') return
  const panel = guide.value?.closest('.modal-content')
  const elements = panel?.querySelectorAll<HTMLElement>('button:not(:disabled), select, a[href], [tabindex="0"]')
  if (!elements?.length) return
  const first = elements[0], last = elements[elements.length - 1]
  if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
  else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
})
</script>

<style scoped>
.billing-guide { --guide-ink:#263c56; --guide-muted:#75869d; --guide-line:#e4eaf2; --guide-fill:#f7f9fc; --guide-active:#edf5ff; --guide-accent:#1677ff; color:var(--guide-ink); }
.guide-intro { font-size:.8125rem; color:var(--guide-muted); line-height:1.7; }
.guide-pages { display:flex; align-items:center; gap:1.4rem; margin:1.25rem 0 1.5rem; border-bottom:1px solid var(--guide-line); }
.guide-pages>button { display:flex; align-items:center; gap:.5rem; padding:.65rem 0; border-bottom:2px solid transparent; font-size:.75rem; color:var(--guide-muted); }
.guide-pages>button.selected { border-color:var(--guide-accent); color:var(--guide-accent); font-weight:600; }
.guide-pages>button.guide-replay { margin-left:auto; gap:.4rem; }
.guide-columns { display:grid; grid-template-columns:1fr 1fr; gap:1.75rem; animation:guide-reveal .38s cubic-bezier(.25,1,.5,1); }
.guide-columns h4 { display:flex; align-items:center; gap:.75rem; font-size:.625rem; letter-spacing:.12em; color:var(--guide-muted); font-weight:600; margin-bottom:.8rem; text-transform:uppercase; }
.guide-columns h4::after { content:''; flex:1; height:1px; background:var(--guide-line); }
.formula-step { display:flex; flex-direction:column; gap:.5rem; width:100%; text-align:left; padding:1rem; margin-bottom:.7rem; border:1px solid var(--guide-line); border-radius:.8rem; background:var(--guide-fill); }
.step-heading { display:flex; align-items:center; gap:.65rem; font-size:.8rem; }
.step-number { font-size:.65rem; font-variant-numeric:tabular-nums; color:var(--guide-muted); }
.formula-step>strong { font-size:.875rem; font-weight:600; }
.step-description { font-size:.75rem; color:var(--guide-muted); line-height:1.65; }
.formula-step,.example-step { transition:background-color .52s cubic-bezier(.25,1,.5,1),border-color .52s cubic-bezier(.25,1,.5,1),transform .52s cubic-bezier(.25,1,.5,1); }
.formula-step.highlighted,.example-step.highlighted { background:var(--guide-active); border-color:var(--guide-accent); transform:translateY(-1px); }
.example-picker { display:flex; flex-direction:column; gap:.4rem; font-size:.7rem; color:var(--guide-muted); }
.example-picker select { width:100%; min-width:0; border-radius:.55rem; border:1px solid var(--guide-line); padding:.55rem 1.8rem .55rem .65rem; color:var(--guide-ink); background:var(--guide-fill); font-size:.75rem; }
.guide-example { min-width:0; }.example-heading { margin:1rem 0; display:flex; flex-direction:column; gap:.35rem; overflow-wrap:anywhere; }.example-heading>strong { font-size:1rem; letter-spacing:-.02em; }.example-heading>span { font-size:.7rem; color:var(--guide-muted); }
.example-calculation { display:flex; flex-direction:column; gap:.5rem; }
.example-step { display:flex; gap:.65rem; border:1px solid transparent; border-radius:.65rem; padding:.75rem; }
.example-step>div { display:flex; flex-direction:column; gap:.3rem; min-width:0; }.example-step span { font-size:.7rem; }.example-step strong { font-size:1rem; font-weight:600; font-variant-numeric:tabular-nums; overflow-wrap:anywhere; }.example-step small { font-size:.65rem; color:var(--guide-muted); }.example-total { border-top-color:var(--guide-line); }.example-total strong { color:var(--guide-accent); font-size:1.4rem; }
.example-note { margin-top:.75rem; font-size:.7rem; line-height:1.7; color:var(--guide-muted); }
.guide-notes { border-top:1px solid var(--guide-line); margin-top:1.25rem; padding-top:1rem; font-size:.72rem; line-height:1.75; color:var(--guide-muted); }
.guide-footer { display:flex; align-items:center; gap:.75rem; margin-top:1.25rem; padding-top:1rem; border-top:1px solid var(--guide-line); }.guide-progress { display:flex; gap:.3rem; margin-right:auto; }.guide-progress i { height:.3rem; width:.3rem; border-radius:1rem; background:var(--guide-line); transition:width .3s; }.guide-progress i.current { width:1.2rem; background:var(--guide-accent); }
.guide-next,.guide-back { display:flex; align-items:center; gap:.4rem; padding:.55rem .85rem; border-radius:.6rem; font-size:.75rem; }.guide-next { color:white; background:#1677ff; }.guide-back { color:var(--guide-muted); }
button:focus-visible,select:focus-visible { outline:2px solid var(--guide-accent); outline-offset:3px; }
:global(.dark .billing-guide) { --guide-ink:#dce7f6; --guide-muted:#9cacc2; --guide-line:#33445a; --guide-fill:#1d2b3e; --guide-active:#213d5c; --guide-accent:#72aaff; }
@keyframes guide-reveal { from { opacity:0; transform:translateY(7px); } to { opacity:1; transform:translateY(0); } }
@media (max-width:639px) { .guide-columns { grid-template-columns:1fr; gap:1rem; }.guide-pages { gap:.75rem; }.guide-pages>button { font-size:.7rem; }.formula-step { padding:.75rem; } }
@media (prefers-reduced-motion:reduce) { .guide-columns { animation:none; }.formula-step,.example-step,.guide-progress i { transition:none; }.formula-step.highlighted,.example-step.highlighted { transform:none; } }
</style>
