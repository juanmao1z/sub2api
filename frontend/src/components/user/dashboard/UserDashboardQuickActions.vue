<!-- @file Compact shortcuts to the existing account workflows. -->
<template>
  <section class="signal-actions" :aria-label="t('dashboard.quickActions')">
    <h2 class="signal-actions-label">{{ t('dashboard.quickActions') }}</h2>
    <div class="signal-actions-list">
      <button type="button" @click="router.push({ path: '/keys', query: { create: '1' } })"><Icon name="key" size="md" /><span>{{ t('dashboard.createApiKey') }}</span><Icon name="arrowRight" size="sm" /></button>
      <button type="button" @click="router.push('/usage')"><Icon name="chart" size="md" /><span>{{ t('dashboard.viewUsage') }}</span><Icon name="arrowRight" size="sm" /></button>
      <button v-if="canUseBatchImage" type="button" @click="router.push('/batch-image')"><Icon name="sparkles" size="md" /><span>{{ t('dashboard.batchImageAgent') }}</span><Icon name="arrowRight" size="sm" /></button>
      <button type="button" @click="router.push('/redeem')"><Icon name="gift" size="md" /><span>{{ t('dashboard.redeemCode') }}</span><Icon name="arrowRight" size="sm" /></button>
    </div>
  </section>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useBatchImageAccess } from '@/composables/useBatchImageAccess'
const router = useRouter()
const { t } = useI18n()
const { canUseBatchImage, refreshBatchImageAccess } = useBatchImageAccess()

onMounted(() => {
  void refreshBatchImageAccess()
})
</script>
