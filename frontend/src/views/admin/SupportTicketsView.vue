<template>
  <AppLayout>
    <div class="space-y-4">
      <div class="flex items-center justify-between">
        <h1 class="page-title">{{ t('support.adminTitle') }}</h1>
        <button class="btn btn-secondary" @click="load">{{ t('common.refresh') }}</button>
      </div>
      <div v-if="loading" class="card p-6 text-center">{{ t('common.loading') }}</div>
      <template v-else>
        <div v-for="item in openTickets" :key="item.ticket.id" class="card cursor-pointer p-4" @click="open(item.ticket.id)">
          <div class="flex justify-between">
            <span><span class="mr-2 rounded bg-gray-100 px-2 py-1 text-xs dark:bg-dark-700">{{ item.ticket.type === 'REFUND' ? t('support.refund') : t('support.suggestion') }}</span>{{ item.ticket.subject }}</span>
            <span class="mr-2 rounded px-2 py-1 text-xs" :class="item.ticket.status === 'OPEN' ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300' : 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-300'">{{ item.ticket.status }}</span>
          </div>
          <p class="mt-2 text-sm text-gray-500">{{ item.ticket.description }}</p>
          <p class="mt-1 text-xs text-gray-400">#{{ item.ticket.id }} · {{ item.user.username || item.user.email || `用户 ${item.user.id}` }} · {{ item.user.email }}</p>
        </div>
        <details v-if="closedTickets.length" class="card overflow-hidden">
          <summary class="cursor-pointer px-4 py-3 font-medium">{{ t('common.close') }} ({{ closedTickets.length }})</summary>
          <div class="space-y-4 border-t border-gray-200 p-4 dark:border-dark-700">
            <div v-for="item in closedTickets" :key="item.ticket.id" class="card cursor-pointer p-4" @click="open(item.ticket.id)">
              <div class="flex justify-between">
                <span><span class="mr-2 rounded bg-gray-100 px-2 py-1 text-xs dark:bg-dark-700">{{ item.ticket.type === 'REFUND' ? t('support.refund') : t('support.suggestion') }}</span>{{ item.ticket.subject }}</span>
                <span class="mr-2 rounded bg-red-100 px-2 py-1 text-xs text-red-700 dark:bg-red-900/30 dark:text-red-300">{{ item.ticket.status }}</span>
              </div>
              <p class="mt-2 text-sm text-gray-500">{{ item.ticket.description }}</p>
              <p class="mt-1 text-xs text-gray-400">#{{ item.ticket.id }} · {{ item.user.username || item.user.email || `用户 ${item.user.id}` }} · {{ item.user.email }}</p>
            </div>
          </div>
        </details>
      </template>
    </div>
    <BaseDialog :show="!!selected" :title="selected?.ticket.subject || ''" width="wide" @close="selected = null">
      <div v-if="selected" class="space-y-4">
        <div class="rounded-lg bg-gray-50 p-3 text-sm dark:bg-dark-800">
          <p><strong>{{ t('support.user') }}:</strong> {{ selected.user.username || selected.user.email || `用户 ${selected.user.id}` }} ({{ selected.user.email }})</p>
          <p v-if="selected.ticket.contact"><strong>{{ t('support.contact') }}:</strong> {{ selected.ticket.contact }}</p>
        </div>
        <div v-for="message in selected.messages" :key="message.id" class="rounded-lg bg-gray-50 p-3 dark:bg-dark-800">
          <div class="text-xs text-gray-500">{{ message.sender_type }} · {{ new Date(message.created_at).toLocaleString() }}</div>
          <p class="whitespace-pre-wrap text-sm">{{ message.body }}</p>
        </div>
        <textarea v-model="reply" class="input w-full" rows="3" />
        <div class="flex flex-wrap justify-end gap-2">
          <select v-model="status" class="input w-auto"><option value="OPEN">OPEN</option><option value="RESOLVED">RESOLVED</option><option value="CLOSED">CLOSED</option></select>
          <button class="btn btn-secondary" @click="send">{{ t('support.reply') }}</button>
        </div>
      </div>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { supportTicketsAPI, type SupportTicketAdminView, type SupportTicketMessage, type SupportTicketStatus } from '@/api/supportTickets'
import { useAppStore } from '@/stores/app'
import { extractI18nErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()
const tickets = ref<SupportTicketAdminView[]>([])
const loading = ref(false)
const selected = ref<(SupportTicketAdminView & { messages: SupportTicketMessage[] }) | null>(null)
const reply = ref('')
const status = ref<SupportTicketStatus>('OPEN')
const openTickets = computed(() => tickets.value.filter(item => item.ticket.status !== 'CLOSED'))
const closedTickets = computed(() => tickets.value.filter(item => item.ticket.status === 'CLOSED'))

async function load() { loading.value = true; try { tickets.value = (await supportTicketsAPI.adminList()).data } catch (e) { appStore.showError(extractI18nErrorMessage(e, t, 'support.errors', t('common.error'))) } finally { loading.value = false } }
async function open(id: number) { try { selected.value = (await supportTicketsAPI.adminGet(id)).data; status.value = selected.value.ticket.status; reply.value = '' } catch (e) { appStore.showError(extractI18nErrorMessage(e, t, 'support.errors', t('common.error'))) } }
async function send() { if (!selected.value) return; try { if (reply.value.trim()) await supportTicketsAPI.adminAddMessage(selected.value.ticket.id, reply.value.trim()); await supportTicketsAPI.adminSetStatus(selected.value.ticket.id, status.value); selected.value = (await supportTicketsAPI.adminGet(selected.value.ticket.id)).data; reply.value = ''; await load() } catch (e) { appStore.showError(extractI18nErrorMessage(e, t, 'support.errors', t('common.error'))) } }
onMounted(load)
</script>
