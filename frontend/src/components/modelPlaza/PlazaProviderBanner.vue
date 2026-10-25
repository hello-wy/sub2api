<template>
  <div class="plaza-provider-banner" :style="{ '--provider-color': bannerColor }">
    <svg class="provider-banner-art" viewBox="0 0 1600 120" preserveAspectRatio="xMidYMid slice" aria-hidden="true">
      <g fill="currentColor" fill-opacity=".06">
        <path d="M150 120 255 -35 585 120Z M360 -25 650 145 910 -30Z M970 145 1250 -30 1470 150Z" />
        <path d="M240 -40 430 140 560 -25Z M790 150 1060 -30 1150 140Z" fill-opacity=".09" />
      </g>
      <g fill="none" stroke="currentColor" stroke-width="1.2" stroke-opacity=".12">
        <path d="m80 130 170-155 90 170 180-165 75 160 190-165 130 170 160-165 120 165 160-170 125 175" />
        <path d="m150-20 75 160 155-175 80 170 165-160 100 165 155-170 110 160 175-150 100 150 160-145" />
        <path d="m165 5 1120 105M210 90 1130 15M320-10l270 150M525-15l320 150M835-10l320 150M1100-10l150 140" />
      </g>
      <g fill="currentColor" fill-opacity=".25">
        <circle cx="250" cy="55" r="2" /><circle cx="520" cy="20" r="2" />
        <circle cx="785" cy="50" r="2" /><circle cx="1060" cy="30" r="2" /><circle cx="1250" cy="88" r="2" />
      </g>
    </svg>
    <div class="provider-banner-copy">
      <div class="provider-banner-title">
        <h2>{{ title }}</h2>
        <span v-if="!loading && !error" class="provider-model-count" role="status" aria-live="polite">{{ t('modelPlaza.gallery.modelCount', { count }) }}</span>
      </div>
      <p>{{ t(platform === 'all' ? 'modelPlaza.gallery.allProvidersDescription' : 'modelPlaza.gallery.providerDescription') }}</p>
    </div>
    <span class="provider-banner-logo" aria-hidden="true">
      <Icon v-if="platform === 'all'" name="grid" size="lg" />
      <PlatformIcon v-else colored :platform="platform" size="lg" />
    </span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import Icon from '@/components/icons/Icon.vue'
import { platformAccentColor } from '@/utils/platformColors'

const props = defineProps<{ platform: string; title: string; count: number; loading: boolean; error?: boolean }>()
const { t } = useI18n()
const bannerColor = computed(() => props.platform === 'all' || props.platform === 'openai' ? '#0d9d78' : platformAccentColor(props.platform))
</script>

<style scoped>
.plaza-provider-banner { position:relative; isolation:isolate; display:flex; align-items:center; justify-content:space-between; gap:1.5rem; min-height:6rem; padding:1.1rem 1.25rem; overflow:hidden; color:#fff; background:#0d9d78; background:color-mix(in srgb,var(--provider-color) 87%,#123f3a); }
.provider-banner-art { position:absolute; z-index:-1; inset:0; width:100%; height:100%; pointer-events:none; }
.provider-banner-copy { min-width:0; }
.provider-banner-title { display:flex; flex-wrap:wrap; align-items:center; gap:.7rem; }
h2 { margin:0; color:#fff; font-size:1.25rem; font-weight:650; line-height:1.35; }
.provider-model-count { padding:.15rem .55rem; border-radius:2rem; background:rgb(255 255 255 / .94); color:#253d39; font-size:.68rem; font-variant-numeric:tabular-nums; line-height:1.4; }
p { margin:.55rem 0 0; color:rgb(255 255 255 / .88); font-size:.78rem; line-height:1.6; }
.provider-banner-logo { display:grid; place-items:center; flex-shrink:0; width:3.5rem; height:3.5rem; border:1px solid rgb(255 255 255 / .1); border-radius:1rem; background:rgb(255 255 255 / .94); color:var(--provider-color); }
.dark .provider-banner-logo { background:rgb(15 23 42 / .88); }
.provider-banner-logo :deep(svg) { width:2.4rem; height:2.4rem; }
@media (max-width:767px) { .plaza-provider-banner { padding:1rem; gap:.75rem; } h2 { font-size:1.1rem; } p { font-size:.72rem; } .provider-banner-logo { width:2.8rem; height:2.8rem; border-radius:.8rem; } .provider-banner-logo :deep(svg) { width:1.9rem; height:1.9rem; } }
</style>
