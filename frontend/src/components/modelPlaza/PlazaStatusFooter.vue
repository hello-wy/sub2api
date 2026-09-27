<template>
  <footer class="plaza-status-footer" :class="status.health">
    <div v-if="hasMetrics" class="status-metrics">
      <span v-if="showThroughput" :title="t('modelPlaza.gallery.throughputHint')">
        <Icon name="trendingUp" size="xs" class="throughput-icon" />
        {{ t('modelPlaza.gallery.throughput') }} <em>{{ number(status.throughput) }} <small>t/s</small></em>
      </span>
      <span :title="t('modelPlaza.gallery.latencyHint')">
        <Icon name="clock" size="xs" class="latency-icon" />
        {{ t('modelPlaza.gallery.latency') }} <em>{{ number(status.latency == null ? null : status.latency / 1000, 2) }}<small>s</small></em>
      </span>
    </div>
    <span v-else class="no-metrics">{{ t('modelPlaza.gallery.noStatus') }}</span>
    <div class="status-history">
      <time v-if="status.updatedAt" :datetime="status.updatedAt" :title="fullTime">{{ shortTime }}</time>
      <span class="history-bars" role="img" :aria-label="historyLabel" :title="historyLabel">
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
const hasMetrics = computed(() => props.status.throughput != null || props.status.latency != null)
const number = (v: number | null, digits = 1) => v == null ? '—' : v.toFixed(digits)
const fullTime = computed(() => props.status.updatedAt ? new Date(props.status.updatedAt).toLocaleString() : '')
const shortTime = computed(() => props.status.updatedAt ? new Date(props.status.updatedAt).toLocaleString(undefined, { month:'2-digit', day:'2-digit', hour:'2-digit', minute:'2-digit' }) : '')
const historyLabel = computed(() => t('modelPlaza.gallery.historyHint', { range: props.status.window, value: props.status.availability == null ? '—' : props.status.availability.toFixed(1) + '%' }))
// Each bar retains the worst health in its window, so brief outages remain visible.
const history = computed(() => props.status.history.length ? plazaHistory(props.status.history) : Array.from({ length:24 }, () => ({ health:'unknown', at:'', endAt:'' })))
</script>
<style scoped>
.plaza-status-footer { display:flex; flex-wrap:wrap; align-items:center; gap:.45rem .6rem; border-top:1px solid var(--plaza-line); padding-top:.75rem; margin-top:.8rem; font-size:.64rem; color:var(--plaza-muted); }
.status-metrics,.status-metrics>span { display:flex; align-items:center; gap:.24rem; white-space:nowrap; }
.status-metrics { gap:.5rem; } em { color:var(--plaza-ink); font-family:Georgia,serif; font-size:.73rem; font-variant-numeric:tabular-nums; } small { font-size:.6rem; }
.throughput-icon { color:#0abb88; width:.75rem; } .latency-icon { color:#508bff; width:.75rem; }
.status-history { display:flex; flex:1; min-width:9rem; align-items:center; justify-content:flex-end; gap:.45rem; }
time { white-space:nowrap; font-size:.56rem; font-style:italic; }
.history-bars { display:flex; flex:1; min-width:4.5rem; max-width:10rem; height:1rem; align-items:center; gap:2px; }
.history-bars>span { flex:1; min-width:1px; height:100%; border-radius:2px; background:#0dc28c; }
.history-bars>.warning { background:#f6b240; } .history-bars>.critical { background:#fb6475; }
.history-bars>.unknown { height:4px; background:var(--plaza-line); }
strong { font-weight:650; font-size:.75rem; font-variant-numeric:tabular-nums; }
.healthy strong { color:#009e73; } .warning strong { color:#b97705; } .critical strong { color:#e44a63; }
:global(.dark .plaza-status-footer.healthy strong) { color:#55d9b1; }
:global(.dark .plaza-status-footer.warning strong) { color:#f4c063; }
:global(.dark .plaza-status-footer.critical strong) { color:#ff8b9e; }
</style>
