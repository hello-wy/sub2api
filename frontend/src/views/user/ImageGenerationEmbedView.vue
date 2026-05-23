<template>
  <AppLayout>
    <section class="image-generation-embed" aria-label="GPT Image Generation">
      <iframe
        :key="frameKey"
        class="image-generation-embed__frame"
        :src="embedUrl"
        :title="t('imageGeneration.title')"
        referrerpolicy="strict-origin-when-cross-origin"
        allow="clipboard-read; clipboard-write; fullscreen"
        sandbox="allow-downloads allow-forms allow-modals allow-popups allow-popups-to-escape-sandbox allow-same-origin allow-scripts"
        @load="isLoading = false"
      />

      <div v-if="isLoading" class="image-generation-embed__loading">
        <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
        <span>{{ t('imageGeneration.loading') }}</span>
      </div>

      <div class="image-generation-embed__actions">
        <button class="image-generation-embed__button" type="button" :title="t('common.refresh')" @click="reloadFrame">
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.8">
            <path stroke-linecap="round" stroke-linejoin="round" d="M16.023 9.348h4.992V4.356m-.803 4.176A8.96 8.96 0 0012 3.75a9 9 0 106.364 15.364" />
          </svg>
        </button>
        <a
          class="image-generation-embed__button"
          :href="embedUrl"
          target="_blank"
          rel="noopener noreferrer"
          :title="t('imageGeneration.openExternal')"
        >
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.8">
            <path stroke-linecap="round" stroke-linejoin="round" d="M13.5 6H18m0 0v4.5M18 6l-6.75 6.75M6 7.5v10.125c0 .621.504 1.125 1.125 1.125h10.125c.621 0 1.125-.504 1.125-1.125V13.5" />
          </svg>
        </a>
      </div>
    </section>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'

const DEFAULT_IMAGE_GENERATION_URL = 'https://gpt-image.solidapi.top/'

const { t } = useI18n()
const frameKey = ref(0)
const isLoading = ref(true)

const embedUrl = computed(() => {
  const configuredUrl = import.meta.env.VITE_IMAGE_GENERATION_URL?.trim()
  return configuredUrl || DEFAULT_IMAGE_GENERATION_URL
})

function reloadFrame() {
  isLoading.value = true
  frameKey.value += 1
}
</script>

<style scoped>
.image-generation-embed {
  position: relative;
  min-height: calc(100dvh - 8rem);
  overflow: hidden;
  border: 1px solid rgb(229 231 235 / 0.9);
  border-radius: 0.75rem;
  background: rgb(255 255 255);
  box-shadow: 0 12px 30px rgb(15 23 42 / 0.06);
}

.dark .image-generation-embed {
  border-color: rgb(55 65 81 / 0.8);
  background: rgb(17 24 39);
  box-shadow: 0 12px 30px rgb(0 0 0 / 0.18);
}

.image-generation-embed__frame {
  display: block;
  width: 100%;
  height: calc(100dvh - 8rem);
  min-height: 680px;
  border: 0;
  background: white;
}

.image-generation-embed__loading {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.75rem;
  background: rgb(255 255 255 / 0.86);
  color: rgb(75 85 99);
  font-size: 0.875rem;
  font-weight: 500;
  backdrop-filter: blur(8px);
}

.dark .image-generation-embed__loading {
  background: rgb(17 24 39 / 0.82);
  color: rgb(209 213 219);
}

.image-generation-embed__actions {
  position: absolute;
  right: 0.75rem;
  top: 0.75rem;
  z-index: 2;
  display: flex;
  gap: 0.5rem;
}

.image-generation-embed__button {
  display: inline-flex;
  height: 2rem;
  width: 2rem;
  align-items: center;
  justify-content: center;
  border: 1px solid rgb(229 231 235 / 0.9);
  border-radius: 0.5rem;
  background: rgb(255 255 255 / 0.9);
  color: rgb(55 65 81);
  box-shadow: 0 8px 20px rgb(15 23 42 / 0.08);
  transition:
    background-color 0.18s ease,
    border-color 0.18s ease,
    color 0.18s ease,
    transform 0.18s ease;
}

.image-generation-embed__button:hover {
  border-color: rgb(156 163 175);
  background: rgb(249 250 251);
  color: rgb(17 24 39);
  transform: translateY(-1px);
}

.dark .image-generation-embed__button {
  border-color: rgb(75 85 99 / 0.8);
  background: rgb(31 41 55 / 0.88);
  color: rgb(209 213 219);
  box-shadow: 0 8px 20px rgb(0 0 0 / 0.2);
}

.dark .image-generation-embed__button:hover {
  border-color: rgb(107 114 128);
  background: rgb(55 65 81);
  color: white;
}

@media (max-width: 768px) {
  .image-generation-embed,
  .image-generation-embed__frame {
    min-height: calc(100dvh - 6rem);
    height: calc(100dvh - 6rem);
  }

  .image-generation-embed {
    border-radius: 0.5rem;
  }
}
</style>
