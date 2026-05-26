<template>
  <AppLayout>
    <section class="image-generation-embed" aria-label="GPT Image Generation">
      <iframe
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
    </section>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'

const DEFAULT_IMAGE_GENERATION_URL = 'https://gpt-image.solidapi.top/'

const { t } = useI18n()
const isLoading = ref(true)

const embedUrl = computed(() => {
  const configuredUrl = import.meta.env.VITE_IMAGE_GENERATION_URL?.trim()
  return configuredUrl || DEFAULT_IMAGE_GENERATION_URL
})
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
