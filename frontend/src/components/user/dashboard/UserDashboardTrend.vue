<!-- @file Total-token trend with an accessible switch to the existing token detail chart. -->
<template>
  <section class="signal-trend">
    <header class="signal-trend-heading"><h3>{{ t('dashboard.tokenUsageTrend') }}</h3><div class="tabs" :aria-label="t('dashboard.tokenUsageTrend')"><button type="button" class="tab" :class="{ 'tab-active': !details }" :aria-pressed="!details" @click="details = false">{{ t('dashboard.totalUsage') }}</button><button type="button" class="tab" :class="{ 'tab-active': details }" :aria-pressed="details" @click="details = true">{{ t('dashboard.tokenDetails') }}</button></div></header>
    <TokenUsageTrend v-if="details" class="signal-token-details" :trend-data="trend" :loading="loading" />
    <div v-else-if="loading" class="h-48 flex items-center justify-center"><LoadingSpinner /></div>
    <div v-else-if="trend.length" class="h-48"><Line :data="chartData" :options="chartOptions" /></div>
    <p v-else class="h-48 flex items-center justify-center text-sm text-gray-500">{{ t('dashboard.noDataAvailable') }}</p>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useMutationObserver } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { Line } from 'vue-chartjs'
import { Chart as ChartJS, CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Filler, type ChartOptions } from 'chart.js'
import type { TrendDataPoint } from '@/types'
import TokenUsageTrend from '@/components/charts/TokenUsageTrend.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Filler)
/** @brief Trend data is supplied by the dashboard date and granularity filters. */
const props = defineProps<{ trend: TrendDataPoint[]; loading: boolean }>()
const { t } = useI18n()
const details = ref(false)
const dark = ref(document.documentElement.classList.contains('dark'))
useMutationObserver(document.documentElement, () => { dark.value = document.documentElement.classList.contains('dark') }, { attributes: true, attributeFilter: ['class'] })
const chartData = computed(() => ({labels:props.trend.map(point => point.date),datasets:[{label:t('dashboard.totalUsage'),data:props.trend.map(point => point.total_tokens),borderColor:dark.value?'#abb8d0':'#424b5c',backgroundColor:dark.value?'#abb8d014':'#424b5c0b',borderWidth:2,pointRadius:2.5,fill:true,tension:0.3}]}))
const chartOptions = computed<ChartOptions<'line'>>(() => ({responsive:true,maintainAspectRatio:false,interaction:{intersect:false,mode:'index'},plugins:{legend:{display:false}},scales:{x:{grid:{display:false},ticks:{color:dark.value?'#a1a1a7':'#737379',font:{size:10}}},y:{beginAtZero:true,grid:{color:dark.value?'#2b2b2e':'#e9e9ec'},ticks:{color:dark.value?'#a1a1a7':'#737379',font:{size:10},callback: value => compact(Number(value))}}}}))
/** @brief Format token counts on the total-usage axis.
 * @param value Tick value in tokens.
 * @return Compact token label.
 */
function compact(value: number): string {
  if (value >= 1e9) return `${(value / 1e9).toFixed(2)}B`
  if (value >= 1e6) return `${(value / 1e6).toFixed(0)}M`
  if (value >= 1e3) return `${(value / 1e3).toFixed(0)}K`
  return String(value)
}
</script>
