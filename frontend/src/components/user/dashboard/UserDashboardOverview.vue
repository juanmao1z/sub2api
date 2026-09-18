<!-- @file Account overview cards and model distribution, backed by the existing dashboard API data. -->
<template>
  <section class="signal-stats" :aria-label="t('dashboard.title')">
    <div class="signal-lead-grid" :class="{ 'signal-lead-grid--simple': isSimple }">
      <article v-if="!isSimple" class="signal-metric signal-metric--balance">
        <p class="signal-label">{{ t('dashboard.balance') }} &nbsp;USD</p>
        <p class="signal-value">${{ balance.toFixed(2) }}</p>
        <div class="signal-metric-foot"><span>{{ t('dashboard.recentTotal') }} / {{ t('dashboard.actual') }}</span><strong>${{ stats.total_actual_cost.toFixed(4) }}</strong><span>{{ t('dashboard.standard') }} ${{ stats.total_cost.toFixed(4) }}</span></div>
      </article>
      <article class="signal-metric">
        <p class="signal-label">{{ t('dashboard.recentTokens') }}</p>
        <p class="signal-value">{{ compact(stats.total_tokens) }} <small>tokens</small></p>
        <dl class="signal-token-split">
          <div><dt>{{ t('dashboard.input') }}</dt><dd>{{ compact(stats.total_input_tokens) }}</dd></div>
          <div><dt>{{ t('dashboard.output') }}</dt><dd>{{ compact(stats.total_output_tokens) }}</dd></div>
          <div><dt>{{ t('dashboard.cache') }}</dt><dd>{{ compact(stats.total_cache_creation_tokens + stats.total_cache_read_tokens) }}</dd></div>
        </dl>
      </article>
      <UserDashboardQuickActions />
    </div>
    <div class="signal-stat-strip signal-stat-strip--models">
      <article class="signal-metric"><p class="signal-label">{{ t('dashboard.todayTokens') }}</p><p class="signal-value">{{ compact(stats.today_tokens) }}</p><div class="signal-metric-foot"><span>{{ t('dashboard.input') }} {{ compact(stats.today_input_tokens) }} / {{ t('dashboard.output') }} {{ compact(stats.today_output_tokens) }}</span><span>{{ t('dashboard.cache') }} {{ compact(stats.today_cache_creation_tokens + stats.today_cache_read_tokens) }}</span></div></article>
      <article class="signal-metric signal-metric--cost"><p class="signal-label">{{ t('dashboard.todayCost') }}</p><p class="signal-value">${{ stats.today_actual_cost.toFixed(4) }}</p><div class="signal-metric-foot"><span>{{ t('dashboard.standard') }}</span><strong>${{ stats.today_cost.toFixed(4) }}</strong></div></article>
      <article class="signal-metric"><p class="signal-label">{{ t('dashboard.todayRequests') }}</p><p class="signal-value">{{ stats.today_requests }}</p><div class="signal-metric-foot"><span>{{ t('dashboard.recentTotal') }}</span><strong>{{ stats.total_requests.toLocaleString() }}</strong></div></article>
      <article class="signal-metric"><p class="signal-label">{{ t('dashboard.apiKeys') }}</p><p class="signal-value">{{ stats.total_api_keys }}</p><div class="signal-metric-foot"><router-link to="/keys">{{ stats.active_api_keys }} {{ t('common.active') }}</router-link></div></article>
      <article class="signal-model-card">
        <h2 class="signal-label">{{ t('dashboard.modelDistribution') }}</h2>
        <p class="signal-model-period">{{ startDate }} / {{ endDate }}</p>
        <div v-if="modelTotal > 0" class="signal-model-summary">
          <div class="signal-model-ring"><Doughnut :data="chartData" :options="chartOptions" /></div>
          <ul class="signal-model-legend"><li v-for="(model, index) in rankedModels.slice(0, 3)" :key="model.model" :title="model.model"><i :style="{ background: colors[index] }" /><span>{{ model.model }}</span><strong>{{ ((model.total_tokens / modelTotal) * 100).toFixed(1) }}%</strong></li></ul>
        </div>
        <p v-else class="signal-label py-4">{{ t('dashboard.noDataAvailable') }}</p>
      </article>
    </div>
    <div class="signal-telemetry">
      <p class="signal-reading"><span class="signal-label">{{ t('dashboard.performance') }}</span><span class="signal-reading-value">{{ compact(stats.rpm) }} RPM / {{ compact(stats.tpm) }} TPM</span></p>
      <p class="signal-reading"><span class="signal-label">{{ t('dashboard.avgResponse') }}</span><span class="signal-reading-value">{{ stats.average_duration_ms >= 1000 ? `${(stats.average_duration_ms / 1000).toFixed(2)}s` : `${stats.average_duration_ms.toFixed(0)}ms` }}</span></p>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Doughnut } from 'vue-chartjs'
import { Chart as ChartJS, ArcElement, Tooltip, Legend, type ChartOptions } from 'chart.js'
import type { UserDashboardStats } from '@/api/usage'
import type { ModelStat } from '@/types'
import UserDashboardQuickActions from './UserDashboardQuickActions.vue'

ChartJS.register(ArcElement, Tooltip, Legend)
/** @brief Statistics use the server's billing totals; model shares use the selected chart period. */
const props = defineProps<{ stats: UserDashboardStats; balance: number; isSimple: boolean; models: ModelStat[]; startDate: string; endDate: string }>()
const { t } = useI18n()
const colors = ['#9bbaf5', '#70b9aa', '#c6b58b', '#b39dda', '#de9ba8', '#88b9cf']
const rankedModels = computed(() => [...props.models].sort((a, b) => b.total_tokens - a.total_tokens))
const modelTotal = computed(() => rankedModels.value.reduce((sum, model) => sum + model.total_tokens, 0))
const chartData = computed(() => ({ labels: rankedModels.value.map(model => model.model), datasets: [{ data: rankedModels.value.map(model => model.total_tokens), backgroundColor: colors, borderWidth: 0 }] }))
const chartOptions: ChartOptions<'doughnut'> = { responsive: true, maintainAspectRatio: false, cutout: '76%', plugins: { legend: { display: false } } }

/** @brief Format finite token and throughput counts using the reference's compact units.
 * @param value Token count or throughput; invalid values are rendered as zero.
 * @return Human-readable count in B, M, K or integer units.
 */
function compact(value: number): string {
  if (!Number.isFinite(value)) return '0'
  if (value >= 1e9) return `${(value / 1e9).toFixed(2)}B`
  if (value >= 1e6) return `${(value / 1e6).toFixed(1)}M`
  if (value >= 1e3) return `${(value / 1e3).toFixed(1)}K`
  return value.toLocaleString()
}
</script>
