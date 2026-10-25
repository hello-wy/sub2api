<template>
  <footer class="plaza-status-footer" :class="status.health">
    <div class="status-metrics">
      <span v-if="showThroughput" class="throughput-metric" :title="t('modelPlaza.gallery.throughputHint')">
        <Icon name="trendingUp" size="xs" class="throughput-icon" />
        <span class="metric-label">{{ t('modelPlaza.gallery.throughput') }}</span> <em>{{ number(status.throughput) }} <small v-if="status.throughput != null">t/s</small></em>
      </span>
      <span class="latency-metric" :title="t('modelPlaza.gallery.latencyHint')">
        <Icon name="clock" size="xs" class="latency-icon" />
        <span class="metric-label">{{ t('modelPlaza.gallery.latency') }}</span> <em>{{ number(status.latency == null ? null : status.latency / 1000, 2) }}<small v-if="status.latency != null">s</small></em>
      </span>
    </div>
    <div class="status-history">
      <time :datetime="status.updatedAt ?? undefined" :title="fullTime">{{ shortTime }}</time>
      <span class="history-bars" role="img" :aria-label="historyLabel" :title="fullTime ? fullTime + ' · ' + historyLabel : historyLabel">
        <span v-for="(point, index) in history" :key="index" :class="point.health"
          :title="point.at ? new Date(point.at).toLocaleString() + (point.endAt !== point.at ? '–' + new Date(point.endAt).toLocaleString() : '') + ' · ' + t('modelPlaza.gallery.health.' + point.health) : t('modelPlaza.gallery.health.unknown')" />
      </span>
      <strong :title="historyLabel">{{ status.availability == null ? '—' : status.availability.toFixed(1) + '%' }}</strong>
    </div>
  </footer>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { plazaHistory, type PlazaStatus } from '@/utils/model-plaza'
const props = defineProps<{ status: PlazaStatus; showThroughput: boolean }>()
const { t } = useI18n()
const number = (v: number | null, digits = 1) => v == null ? '—' : v.toFixed(digits)
const fullTime = computed(() => props.status.updatedAt ? new Date(props.status.updatedAt).toLocaleString() : '')
const shortTime = computed(() => {
  if (!props.status.updatedAt) return ''
  const date = new Date(props.status.updatedAt)
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
})
const historyLabel = computed(() => t('modelPlaza.gallery.historyHint', { range: props.status.window, value: props.status.availability == null ? '—' : props.status.availability.toFixed(1) + '%' }))
const history = computed(() => plazaHistory(props.status))
</script>
<style scoped>
.plaza-status-footer { display:grid; grid-template-columns:max-content minmax(0,1fr); align-items:center; column-gap:.6rem; min-width:0; border-top:1px solid var(--plaza-soft-line); padding-top:.65rem; font-size:clamp(.625rem,2.8cqi,.68rem); line-height:1.4; color:var(--plaza-muted); }
.status-metrics,.status-metrics>span { display:flex; align-items:center; gap:.25rem; white-space:nowrap; }
.status-metrics { display:grid; grid-template-columns:6.6em 7.2em; gap:.5rem; }
.status-metrics>span { min-width:0; overflow:hidden; }
.status-metrics:has(>span:only-child) { grid-template-columns:7.2em; }
em { color:var(--plaza-ink); font-family:Georgia,'Times New Roman',serif; font-size:clamp(.6875rem,3.2cqi,.78rem); font-variant-numeric:tabular-nums; }
small { font-size:inherit; }
.throughput-icon { color:#0abb88; width:.75rem; } .latency-icon { color:#508bff; width:.75rem; }
.status-history { display:grid; grid-template-columns:minmax(0,1fr) 8.875rem 6ch; min-width:0; align-items:center; gap:.5rem; white-space:nowrap; }
time { min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; text-align:right; font-size:.6rem; opacity:.6; font-family:Georgia,'Times New Roman',serif; font-style:italic; }
.history-bars { display:grid; grid-template-columns:repeat(24,minmax(0,1fr)); direction:ltr; width:100%; max-width:8.875rem; justify-self:end; min-width:0; height:1rem; align-items:center; gap:2px; }
.history-bars>span { min-width:0; height:100%; border-radius:2px; background:#0dbb86; }
.history-bars>.warning { background:#f6b240; } .history-bars>.critical { background:#fb6475; }
.history-bars>.unknown { background:#9ca3af; }
strong { text-align:right; font-weight:600; font-size:.75rem; font-variant-numeric:tabular-nums; }
.healthy strong { color:var(--plaza-green); } .warning strong { color:var(--plaza-amber); } .critical strong { color:var(--plaza-red); }
@container (max-width:28rem) {
  .plaza-status-footer { column-gap:.4rem; }
  .status-metrics { grid-template-columns:5.8em 4.8em; gap:.35rem; }
  .status-metrics:has(>span:only-child) { grid-template-columns:4.8em; }
  .status-metrics>span { gap:.2rem; }
  .metric-label { position:absolute; width:1px; height:1px; padding:0; overflow:hidden; clip-path:inset(50%); white-space:nowrap; }
  .status-history { grid-template-columns:minmax(0,1fr) 6ch; gap:.5rem; }
  time { display:none; }
  .throughput-icon,.latency-icon { width:.65rem; height:.65rem; }
}
</style>
