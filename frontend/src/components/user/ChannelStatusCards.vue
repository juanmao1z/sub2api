<!-- @file Grouped channel health cards using the existing monitor matrix and latency privacy formatter. -->
<template>
  <div class="channel-card-groups">
    <section v-for="group in platforms" :key="group.platform" class="channel-card-platform" :class="{ 'channel-card-platform--multiple': group.rows.length > 1 }">
      <header><h2>{{ group.platform.toUpperCase() }}</h2><span>{{ group.rows.length }}</span></header>
      <div class="channel-card-grid">
        <article v-for="(row, index) in group.rows" :key="`${row.group_id}-${row.model}-${index}`" class="channel-health-card">
          <header><span class="channel-platform-mark">{{ group.platform.substring(0, 1).toUpperCase() }}</span><div><h3>{{ row.group_name || row.model || row.platform }}</h3><span class="badge badge-gray">{{ row.platform }}</span></div></header>
          <dl>
            <div><dt>{{ t('channelMonitorV2.metrics.cacheRate') }}</dt><dd>{{ formatMonitorPercent(row.metrics.cache_rate) }}</dd></div>
            <div><dt>{{ t('channelMonitorV2.metrics.successRate') }}</dt><dd :class="'health-' + row.health.overall">{{ formatMonitorPercent(1 - row.metrics.error_rate) }}</dd></div>
            <div><dt>{{ t('channelMonitorV2.metrics.ttftP50') }}</dt><dd>{{ formatMonitorMs(row.metrics.ttft.p50_ms) }}</dd></div>
          </dl>
          <div class="channel-history-heading"><span>{{ t('channelMonitorV2.cardHistory', { count: row.buckets.length }) }}</span><span>{{ healthLabel }}</span></div>
          <div class="channel-history" :aria-label="t('channelMonitorV2.cardHistory', { count: row.buckets.length })">
            <div v-for="bucket in row.buckets" :key="bucket.bucket_start" :class="'health-' + bucketState(bucket)" :title="`${bucket.bucket_start} · ${formatMonitorPercent(1 - bucket.metrics.error_rate)}`" :style="{ height: `${bucketState(bucket) !== 'unknown' ? Math.max(25, (1 - bucket.metrics.error_rate) * 100) : 18}%` }" />
          </div>
          <div class="channel-history-heading"><span>{{ t('channelMonitorV2.cardPast') }}</span><span>{{ t('channelMonitorV2.cardNow') }}</span></div>
          <p v-if="showThroughput" class="channel-throughput">{{ row.metrics.rpm.toFixed(1) }} RPM / {{ Math.round(row.metrics.tpm).toLocaleString() }} TPM</p>
        </article>
      </div>
    </section>
    <p v-if="!rows.length" class="empty-state">{{ t('common.noData') }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { MonitorMatrixRow, MonitorMatrixBucket, HealthState } from '@/api/channelMonitorV2'
import { formatMonitorMs, formatMonitorPercent } from '@/features/channel-monitor-v2/monitorFormat'

/** @brief Matrix rows remain ordered within each platform; no requests or settings are changed here. */
const props = defineProps<{ rows: MonitorMatrixRow[]; healthMode: 'overall' | 'success' | 'ttft' | 'cache'; showThroughput: boolean }>()
const { t } = useI18n()
const platforms = computed(() => {
  const groups = new Map<string, MonitorMatrixRow[]>()
  for (const row of props.rows) groups.set(row.platform, [...(groups.get(row.platform) || []), row])
  return [...groups].map(([platform, rows]) => ({ platform, rows }))
})
const healthLabel = computed(() => t(`channelMonitorV2.healthMode.${props.healthMode}`))

/** @brief Use the selected health state because user-facing request counts are redacted to zero.
 * @param bucket Server-provided interval health and metrics.
 * @return Health state used to color a single interval.
 */
function bucketState(bucket: MonitorMatrixBucket): HealthState {
  if (props.healthMode === 'success') return bucket.health.error_rate
  return bucket.health[props.healthMode] || 'unknown'
}
</script>
