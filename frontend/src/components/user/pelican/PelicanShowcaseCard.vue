<template>
  <article
    ref="cardRef"
    class="group relative min-w-0 overflow-hidden rounded-lg border border-gray-200/80 bg-white transition-colors hover:border-primary-300 focus-within:border-primary-400 dark:border-dark-700 dark:bg-dark-900/40 dark:hover:border-primary-500/60"
    data-testid="pelican-showcase-card"
  >
    <div ref="thumbnailRef" class="relative aspect-[4/3] overflow-hidden border-b border-gray-100 bg-gray-50 dark:border-dark-700/70 dark:bg-dark-900/40">
      <iframe
        v-if="body?.status === 'ready'"
        :srcdoc="body.html"
        class="pointer-events-none absolute left-0 top-0 origin-top-left border-0"
        :style="thumbnailStyle"
        tabindex="-1"
        sandbox="allow-scripts"
        referrerpolicy="no-referrer"
        :title="label"
      />
      <div v-else class="absolute inset-0 flex items-center justify-center p-2 text-center text-[11px] leading-relaxed text-gray-400 dark:text-gray-500">
        <span v-if="!body || body.status === 'loading'" class="animate-pulse">{{ t('pelicanShowcase.itemLoading') }}</span>
        <span v-else-if="body.status === 'invalid'">{{ t('pelicanShowcase.invalidHtml') }}</span>
        <span v-else class="text-red-500 dark:text-red-400">{{ t('pelicanShowcase.itemLoadError') }}</span>
      </div>
    </div>

    <div class="space-y-1.5 p-2">
      <p class="truncate font-mono text-[11px] font-semibold leading-4 text-gray-900 dark:text-gray-100" :title="item.model_id">
        {{ item.model_id || '—' }}
      </p>
      <p class="truncate text-[10px] tabular-nums leading-4 text-gray-500 dark:text-gray-400" :title="formatDateTimeToMinute(item.generated_at)">{{ formatDateTimeToMinute(item.generated_at) }}</p>
      <div class="flex min-w-0 flex-wrap items-center gap-1 text-[10px] leading-4">
        <span class="max-w-full truncate rounded bg-primary-50 px-1 py-0.5 font-medium text-primary-700 dark:bg-primary-900/20 dark:text-primary-300" data-testid="showcase-source">{{ t(item.source_scope === 'group' ? 'pelicanShowcase.sourceGroup' : 'pelicanShowcase.sourceAccount') }}</span>
        <span
          v-if="effortLabel"
          class="max-w-full truncate rounded bg-gray-100 px-1 py-0.5 font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-300"
          :title="effortLabel"
        >
          {{ effortLabel }}
        </span>
      </div>
      <p class="truncate text-[10px] tabular-nums leading-4 text-gray-500 dark:text-gray-400" :title="durationLabel">{{ durationLabel }}</p>
    </div>

    <!-- The iframe is not interactive content of a button, so a full-card overlay opens the preview. -->
    <button
      type="button"
      class="absolute inset-0 z-10 rounded-lg focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-primary-500"
      :aria-label="`${label} · ${t('pelicanShowcase.preview')}`"
      :title="`${label} · ${formatDateTimeToMinute(item.generated_at)} · ${durationLabel}`"
      @click="emit('open')"
    />
  </article>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useResizeObserver } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import type { PelicanShowcaseItem } from '@/api/pelicanShowcase'
import { formatDateTimeToMinute } from '@/utils/format'
import { pelicanDurationLabel, pelicanEffortLabel, type PelicanBody } from './pelicanShowcaseFormat'

const props = defineProps<{
  item: PelicanShowcaseItem
  groupName: string
  body?: PelicanBody
}>()

const emit = defineEmits<{
  (e: 'visible'): void
  (e: 'open'): void
}>()

const { t } = useI18n()
const cardRef = ref<HTMLElement | null>(null)
const thumbnailRef = ref<HTMLElement | null>(null)
const thumbnailWidth = ref(0)
let observer: IntersectionObserver | null = null

const label = computed(() => `${props.groupName} · ${props.item.model_id || '—'}`)
const effortLabel = computed(() => pelicanEffortLabel(t, props.item.reasoning_effort))
const durationLabel = computed(() => pelicanDurationLabel(t, props.item.latency_ms))

// Render the original at a stable viewport, then scale the whole result into its thumbnail.
const thumbnailStyle = computed(() => ({
  width: '800px',
  height: '600px',
  transform: `scale(${thumbnailWidth.value / 800})`,
}))
useResizeObserver(thumbnailRef, ([entry]) => {
  if (entry) thumbnailWidth.value = entry.contentRect.width
})

// HTML bodies load only once a card nears the viewport, so a long gallery costs nothing up front.
onMounted(() => {
  if (typeof IntersectionObserver === 'undefined' || !cardRef.value) {
    emit('visible')
    return
  }
  observer = new IntersectionObserver((entries) => {
    if (entries.some((entry) => entry.isIntersecting)) {
      emit('visible')
      observer?.disconnect()
      observer = null
    }
  }, { rootMargin: '200px' })
  observer.observe(cardRef.value)
})

onBeforeUnmount(() => observer?.disconnect())
</script>
