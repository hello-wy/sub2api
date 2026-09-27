<template>
  <section class="plaza-filter-section" :aria-label="title">
    <h3><span>{{ title }}</span></h3>
    <div class="filter-options" :class="{ 'has-more': collapsible && !expanded }">
      <button
        v-for="option in visibleOptions" :key="option.value" type="button"
        :class="{ selected: modelValue === option.value }" :aria-pressed="modelValue === option.value"
        :title="option.label" @click="$emit('update:modelValue', option.value)"
      >
        <PlatformIcon v-if="option.platform" :platform="option.platform" size="sm" :style="{ color: platformAccentColor(option.platform) }" />
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
.plaza-filter-section { margin-top: 1.8rem; }
h3 { display:flex; align-items:center; gap:.65rem; margin-bottom:1rem; color:var(--plaza-ink); font-size:.9375rem; font-weight:600; }
h3::before,h3::after { content:''; height:1px; background:var(--plaza-line); }
h3::before { width:1.75rem; } h3::after { flex:1; }
.filter-options { position:relative; display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:.5rem; }
.filter-options button { display:flex; align-items:center; gap:.35rem; min-width:0; min-height:2.5rem; padding:.45rem .55rem; border:1px solid var(--plaza-line); border-radius:.65rem; background:var(--plaza-button); color:var(--plaza-ink); font-size:.875rem; font-weight:500; transition:background .18s,border-color .18s; }
.filter-options button:hover { background:var(--plaza-selected); border-color:#91baff; }
.filter-options button.selected { border-color:transparent; background:var(--plaza-selected); color:var(--plaza-blue); }
.option-label { flex:1; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; text-align:left; }
.option-count { flex-shrink:0; padding:0 .35rem; min-width:1.35rem; border:1px solid var(--plaza-line); border-radius:1rem; background:var(--plaza-button); color:var(--plaza-ink); font-size:.75rem; text-align:center; font-variant-numeric:tabular-nums; }
.has-more::after { content:''; pointer-events:none; position:absolute; inset:auto 0 0; height:2.6rem; background:linear-gradient(transparent,var(--plaza-rail)); }
.filter-more { display:flex; align-items:center; justify-content:center; gap:.4rem; width:100%; margin-top:.65rem; color:var(--plaza-muted); font-size:.8125rem; }
button:focus-visible { outline:2px solid #1677ff; outline-offset:3px; }
</style>
