<template>
  <AppLayout>
    <div>
      <div
        data-testid="recharge-frame-shell"
        class="relative min-h-[calc(100vh-7rem)] overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-soft ring-1 ring-black/5 dark:border-dark-700 dark:bg-dark-900 dark:ring-white/5"
      >
        <iframe
          class="block h-[calc(100vh-7rem)] min-h-[820px] w-full border-0 bg-white"
          :src="PAYMENT_URL"
          :title="t('recharge.iframeTitle')"
          referrerpolicy="no-referrer-when-downgrade"
          loading="eager"
        />

        <button
          data-testid="recharge-open-window"
          type="button"
          class="absolute right-3 top-3 inline-flex h-9 shrink-0 items-center justify-center gap-2 rounded-lg border border-gray-200 bg-white/95 px-3 text-sm font-medium text-gray-700 shadow-lg shadow-gray-900/10 backdrop-blur transition-colors hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 dark:border-dark-600 dark:bg-dark-800/95 dark:text-gray-200 dark:hover:bg-dark-700"
          @click="openInNewWindow"
        >
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
            <path stroke-linecap="round" stroke-linejoin="round" d="M13.5 6H18m0 0v4.5M18 6l-7.5 7.5M6 7.5v10.125C6 18.246 6.504 18.75 7.125 18.75h10.125c.621 0 1.125-.504 1.125-1.125V13.5" />
          </svg>
          <span>{{ t('recharge.openInNewWindow') }}</span>
        </button>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'

const PAYMENT_URL = 'https://pay.ldxp.cn/shop/1WGCPCG0'

const { t } = useI18n()

function openInNewWindow() {
  window.open(PAYMENT_URL, '_blank', 'noopener,noreferrer')
}
</script>
