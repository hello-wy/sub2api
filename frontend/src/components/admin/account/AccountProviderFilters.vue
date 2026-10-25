<template>
  <div
    v-if="providers.length"
    class="account-provider-shortcuts"
    role="group"
    :aria-label="t('admin.accounts.providerShortcuts')"
  >
    <button type="button" :class="{ active: !modelValue }" :aria-pressed="!modelValue" @click="selectProvider('')">
      <Icon name="grid" size="md" />
      {{ t('admin.accounts.allPlatforms') }}
      <span class="provider-count">{{ total }}</span>
    </button>
    <button
      v-for="provider in providers"
      :key="provider.platform"
      type="button"
      :class="{ active: modelValue === provider.platform }"
      :aria-pressed="modelValue === provider.platform"
      @click="selectProvider(provider.platform)"
    >
      <span class="inline-flex" :style="{ color: platformAccentColor(provider.platform) }" aria-hidden="true">
        <PlatformIcon :platform="provider.platform" size="md" />
      </span>
      {{ platformLabel(provider.platform) }}
      <span class="provider-count">{{ provider.count }}</span>
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import { platformAccentColor, platformLabel } from '@/utils/platformColors'
import type { AccountPlatformCount } from '@/api/admin/accounts'

const props = defineProps<{ modelValue: string; platformCounts: AccountPlatformCount[] }>()
const emit = defineEmits<{ 'update:modelValue': [platform: string]; change: [] }>()
const { t } = useI18n()
const providers = computed(() => props.platformCounts.filter(provider => provider.count > 0))
const total = computed(() => providers.value.reduce((sum, provider) => sum + provider.count, 0))

function selectProvider(platform: string) {
  if (props.modelValue === platform) return
  emit('update:modelValue', platform)
  emit('change')
}
</script>

<style scoped>
.account-provider-shortcuts { display:flex; flex-wrap:wrap; gap:.4rem;  }
.account-provider-shortcuts button { display:inline-flex; align-items:center; gap:.45rem; min-height:2.15rem; padding:.35rem .75rem; border:1px solid #e5e7eb; border-radius:.6rem; background:#fff; color:#4b5563; font-size:.8rem; font-weight:500; transition:background .15s,border-color .15s; }
.account-provider-shortcuts button:hover { background:#f9fafb; border-color:#cbd5e1; }
.account-provider-shortcuts button.active { border-color:#bfdbfe; background:#eff6ff; color:#2563eb; }
.provider-count { min-width:1.25rem; padding:0 .3rem; border-radius:1rem; background:rgb(100 116 139 / .08); font-size:.7rem; font-variant-numeric:tabular-nums; text-align:center; }
.account-provider-shortcuts button:focus-visible { outline:2px solid #3b82f6; outline-offset:2px; }
:global(.dark .account-provider-shortcuts button) { border-color:#374151; background:#1f2937; color:#cbd5e1; }
:global(.dark .account-provider-shortcuts button:hover) { border-color:#64748b; background:#273449; }
:global(.dark .account-provider-shortcuts button.active) { border-color:#31599d; background:#1e3a5f; color:#93c5fd; }
@media (prefers-reduced-motion:reduce) { .account-provider-shortcuts button { transition:none; } }
</style>
