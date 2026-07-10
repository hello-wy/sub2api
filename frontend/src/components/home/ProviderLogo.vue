<template>
  <svg
    class="provider-logo"
    :class="providerClass"
    viewBox="0 0 24 24"
    fill="currentColor"
    aria-hidden="true"
  >
    <path v-for="path in paths" :key="path" :d="path" />
  </svg>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import anthropicSource from '@lobehub/icons/es/Anthropic/components/Mono.js?raw'
import claudeSource from '@lobehub/icons/es/Claude/components/Mono.js?raw'
import deepSeekSource from '@lobehub/icons/es/DeepSeek/components/Mono.js?raw'
import geminiSource from '@lobehub/icons/es/Gemini/components/Mono.js?raw'
import grokSource from '@lobehub/icons/es/Grok/components/Mono.js?raw'
import kimiSource from '@lobehub/icons/es/Kimi/components/Mono.js?raw'
import miniMaxSource from '@lobehub/icons/es/Minimax/components/Mono.js?raw'
import openAiSource from '@lobehub/icons/es/OpenAI/components/Mono.js?raw'
import qwenSource from '@lobehub/icons/es/Qwen/components/Mono.js?raw'

const props = defineProps<{ provider: string }>()

const providerSources: Record<string, string> = {
  OpenAI: openAiSource,
  Anthropic: anthropicSource,
  Claude: claudeSource,
  DeepSeek: deepSeekSource,
  Gemini: geminiSource,
  Grok: grokSource,
  Kimi: kimiSource,
  MiniMax: miniMaxSource,
  Qwen: qwenSource
}

const providerClass = 'text-[#06111f] dark:text-white'
const paths = computed(() => Array.from(
  providerSources[props.provider]?.matchAll(/d: "([^"]+)"/g) ?? [],
  (match) => match[1]
))
</script>

<style scoped>
.provider-logo {
  display: inline-flex;
  height: 1em;
  width: 1em;
}
</style>
