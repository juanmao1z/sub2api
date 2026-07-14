<template>
  <section class="pb-8 pt-4 text-center" aria-live="polite">
    <div class="flex items-center justify-center gap-2">
      <span
        data-testid="status-dot"
        aria-hidden="true"
        class="h-2 w-2 rounded-full"
        :class="
          available
            ? 'bg-emerald-500 shadow-[0_0_10px_rgba(16,185,129,0.65)]'
            : 'bg-gray-300 dark:bg-dark-600'
        "
      />
      <p class="text-sm font-medium text-gray-500 dark:text-dark-400">
        {{ t('home.realtimeConcurrency.title') }}
      </p>
    </div>
    <p class="mt-3 text-5xl font-bold text-gray-900 dark:text-white">
      {{ displayValue }}
    </p>
    <p class="mt-2 text-xs text-gray-400 dark:text-dark-500">
      {{ t('home.realtimeConcurrency.refreshHint') }}
    </p>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const props = defineProps<{ current: number | null; available: boolean }>()
const { t } = useI18n()
const displayValue = computed(() =>
  props.available && props.current !== null ? String(props.current) : '--',
)
</script>
