<template>
  <section class="plaza-filter-section" :aria-label="title">
    <h3><span>{{ title }}</span></h3>
    <div class="filter-options" :class="{ 'has-more': collapsible && !expanded }">
      <button
        v-for="option in visibleOptions" :key="option.value" type="button"
        :class="{ selected: modelValue === option.value }" :aria-pressed="modelValue === option.value"
        :title="option.label" @click="$emit('update:modelValue', option.value)"
      >
        <span v-if="option.platform" class="option-icon" :style="{ color: platformAccentColor(option.platform) }" aria-hidden="true"><PlatformIcon colored :platform="option.platform" size="sm" /></span>
        <span class="option-label">{{ option.label }}</span>
        <span v-if="option.badge != null" class="option-count">{{ option.badge }}</span>
      </button>
    </div>
    <button v-if="collapsible" class="filter-more" type="button" :aria-expanded="expanded" @click="expanded = !expanded">
      <Icon name="chevronDown" size="xs" :class="{ 'rotate-180': expanded }" />
      {{ t(expanded ? 'modelPlaza.gallery.less' : 'modelPlaza.gallery.more') }}
    </button>
    <slot />
  </section>
</template>
<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import { platformAccentColor } from '@/utils/platformColors'
export interface PlazaFilterOption { value: string; label: string; badge?: string | number; platform?: string }
const props = defineProps<{ title: string; options: PlazaFilterOption[]; modelValue: string }>()
defineEmits<{ 'update:modelValue': [value: string] }>()
const { t } = useI18n()
const expanded = ref(false)
const collapsible = computed(() => props.options.length > 10)
const visibleOptions = computed(() => expanded.value ? props.options : props.options.slice(0, 10))
</script>
<style scoped>
.plaza-filter-section { padding:1.6rem 0 .3rem; }
.plaza-filter-section:last-child { padding-bottom:0; }
h3 { display:flex; align-items:center; gap:.55rem; margin:0 0 .9rem; color:var(--plaza-ink); font-size:.875rem; font-weight:600; }
h3::before,h3::after { content:''; height:1px; background:var(--plaza-soft-line); }
h3::before { flex:0 0 2.4rem; }
h3::after { flex:1; }
h3 span { flex-shrink:0; }
.filter-options { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:.3rem; position:relative; }
.filter-options button { display:flex; align-items:center; justify-content:center; gap:.3rem; min-width:0; min-height:2rem; padding:.25rem .5rem; border:1px solid var(--plaza-soft-line); border-radius:.65rem; background:var(--plaza-surface); color:var(--plaza-muted); font-size:.8rem; font-weight:550; transition:background .18s,border-color .18s; }
.filter-options button:hover { border-color:var(--plaza-hover-line); color:var(--plaza-ink); background:var(--plaza-subtle); }
.filter-options button.selected { border-color:transparent; background:var(--plaza-subtle); color:var(--plaza-blue); }
.option-icon { display:inline-flex; }
.option-icon :deep(svg) { flex-shrink:0; width:1rem; height:1rem; }
.option-label { min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.option-count { flex-shrink:0; min-width:1.35rem; padding:0 .4rem; border:1px solid var(--plaza-line); border-radius:1rem; background:var(--plaza-surface); color:var(--plaza-ink); font-size:.7rem; font-weight:400; text-align:center; line-height:1.35; font-variant-numeric:tabular-nums; }
.filter-options.has-more::after { content:''; position:absolute; inset:auto 0 -.15rem; height:.85rem; background:linear-gradient(transparent,var(--plaza-surface)); pointer-events:none; }
.filter-more { display:flex; align-items:center; justify-content:center; gap:.2rem; width:100%; margin-top:.7rem; color:var(--plaza-muted); font-size:.72rem; }
.filter-more:hover { color:var(--plaza-blue); }
button:focus-visible { outline:2px solid var(--plaza-blue); outline-offset:3px; }
@media (prefers-reduced-motion:reduce) { .filter-options button { transition:none; } }
</style>
