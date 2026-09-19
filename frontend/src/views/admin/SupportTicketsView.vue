<template>
  <AppLayout>
    <div class="ticket-center ticket-admin">
      <header class="ticket-toolbar">
        <div><h1>{{ t('support.adminTitle') }}</h1><p>{{ t('support.adminDescription') }}</p></div>
        <div class="ticket-tools">
          <label class="ticket-search"><Icon name="search" size="sm" aria-hidden="true" /><input v-model="query" type="search" :aria-label="t('support.adminSearch')" :placeholder="t('support.adminSearch')" /></label>
          <select v-model="statusFilter" class="ticket-filter" :aria-label="t('support.filterStatus')"><option value="ALL">{{ t('support.allTickets') }}</option><option v-for="status in statuses" :key="status" :value="status">{{ t(`support.status.${status}`) }}</option></select>
          <button type="button" class="ticket-icon-button" :aria-label="t('common.refresh')" :title="t('common.refresh')" :disabled="loading || busy" @click="refresh"><Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" /></button>
        </div>
      </header>
      <div class="ticket-workspace">
        <aside class="ticket-history" :aria-label="t('support.allTickets')">
          <div class="ticket-history-heading"><h2>{{ t('support.allTickets') }}</h2><span class="ticket-count">{{ t('support.ticketCount', { count: tickets.length }) }}</span></div>
          <div class="ticket-status-filters">
            <button v-for="status in statuses" :key="status" type="button" :aria-pressed="statusFilter === status" @click="statusFilter = statusFilter === status ? 'ALL' : status"><span class="ticket-dot" :class="`ticket-dot-${status.toLowerCase()}`"></span>{{ t(`support.status.${status}`) }}<span>{{ tickets.filter(item => item.ticket.status === status).length }}</span></button>
          </div>
          <div class="ticket-list" :aria-busy="loading">
            <div v-if="loading && !tickets.length" class="ticket-list-empty" role="status">{{ t('common.loading') }}</div>
            <div v-else-if="listError" class="ticket-list-empty" role="alert"><Icon name="refresh" size="lg" /><p>{{ t('support.loadFailed') }}</p><button type="button" class="ticket-button" @click="load">{{ t('support.retry') }}</button></div>
            <template v-else-if="ticketGroups.length">
              <section v-for="group in ticketGroups" :key="group.status" class="ticket-group" :data-status="group.status">
                <h3><button :id="`admin-ticket-group-heading-${group.status}`" type="button" class="ticket-group-toggle" :aria-expanded="expandedGroups[group.status]" :aria-controls="`admin-ticket-group-items-${group.status}`" @click="expandedGroups[group.status] = !expandedGroups[group.status]">
                  <Icon name="chevronRight" size="sm" class="ticket-group-chevron" :class="{ 'ticket-group-chevron-open': expandedGroups[group.status] }" aria-hidden="true" /><span class="ticket-dot" :class="`ticket-dot-${group.status.toLowerCase()}`"></span><span>{{ t(`support.status.${group.status}`) }}</span><span class="ticket-group-count">{{ group.items.length }}</span>
                </button></h3>
                <div v-show="expandedGroups[group.status]" :id="`admin-ticket-group-items-${group.status}`" role="region" :aria-labelledby="`admin-ticket-group-heading-${group.status}`">
                  <button v-for="item in group.items" :key="item.ticket.id" type="button" class="ticket-list-item" :class="{ 'ticket-list-item-active': activeTicketId === item.ticket.id }" :aria-pressed="activeTicketId === item.ticket.id" :disabled="busy" @click="openTicket(item.ticket.id)">
                    <span class="ticket-list-meta"><span>#{{ item.ticket.id }} · {{ t(item.ticket.type === 'REFUND' ? 'support.refund' : 'support.suggestion') }}</span><time :datetime="item.ticket.updated_at">{{ formatDate(item.ticket.updated_at, true) }}</time></span>
                    <span class="ticket-list-subject">{{ item.ticket.subject }}</span><span class="ticket-list-description">{{ item.ticket.description }}</span>
                    <span class="ticket-list-owner"><Icon name="user" size="xs" />{{ item.user.username || item.user.email || `#${item.user.id}` }}</span>
                  </button>
                </div>
              </section>
            </template>
            <div v-else class="ticket-list-empty"><span class="ticket-empty-icon"><Icon :name="tickets.length ? 'search' : 'chat'" size="lg" /></span><h3>{{ t(tickets.length ? 'support.noMatches' : 'support.empty') }}</h3><p>{{ t(tickets.length ? 'support.changeFilters' : 'support.adminEmptyHint') }}</p><button v-if="tickets.length" type="button" class="ticket-button" @click="resetFilters">{{ t('support.resetFilters') }}</button></div>
          </div>
        </aside>
        <section ref="detailPanel" class="ticket-detail" :aria-label="t('support.details')" :aria-busy="detailLoading">
          <div v-if="detailLoading" class="ticket-detail-empty" role="status"><Icon name="refresh" size="lg" class="animate-spin" /><p>{{ t('common.loading') }}</p></div>
          <div v-else-if="detailError" class="ticket-detail-empty" role="alert"><span class="ticket-empty-icon"><Icon name="chat" size="lg" /></span><h2>{{ t('support.loadFailed') }}</h2><button type="button" class="ticket-button" @click="activeTicketId !== null && openTicket(activeTicketId)">{{ t('support.retry') }}</button></div>
          <template v-else-if="selected">
            <header class="ticket-panel-heading">
              <div><div class="ticket-detail-meta"><span>#{{ selected.ticket.id }} · {{ t(selected.ticket.type === 'REFUND' ? 'support.refund' : 'support.suggestion') }}</span><span class="ticket-status"><span class="ticket-dot" :class="`ticket-dot-${selected.ticket.status.toLowerCase()}`"></span>{{ t(`support.status.${selected.ticket.status}`) }}</span></div><h2>{{ selected.ticket.subject }}</h2><p>{{ formatDate(selected.ticket.created_at) }}</p></div>
            </header>
            <div class="ticket-admin-controls">
              <div class="ticket-owner"><span class="ticket-empty-icon"><Icon name="user" size="md" /></span><div><strong>{{ selected.user.username || selected.user.email || `#${selected.user.id}` }}</strong><span>{{ selected.user.email }} · #{{ selected.user.id }}</span></div></div>
              <form class="ticket-status-form" @submit.prevent="saveStatus"><label for="admin-ticket-status">{{ t('support.manageStatus') }}</label><div><select id="admin-ticket-status" v-model="nextStatus" class="ticket-filter" :disabled="busy"><option v-for="status in statuses" :key="status" :value="status">{{ t(`support.status.${status}`) }}</option></select><button type="submit" class="ticket-button" :disabled="busy || nextStatus === selected.ticket.status">{{ t(savingStatus ? 'support.savingStatus' : 'support.saveStatus') }}</button></div></form>
            </div>
            <div ref="conversation" class="ticket-conversation">
              <div v-if="selected.ticket.order_id || selected.ticket.contact" class="ticket-order-info"><span v-if="selected.ticket.order_id">{{ t('support.orderId') }} · {{ selected.ticket.order_id }}</span><span v-if="selected.ticket.contact">{{ t('support.contact') }} · {{ selected.ticket.contact }}</span></div>
              <article class="ticket-message ticket-message-user"><div class="ticket-message-meta"><strong>{{ t('support.user') }}</strong><time :datetime="selected.ticket.created_at">{{ formatDate(selected.ticket.created_at) }}</time><span>{{ t('support.originalMessage') }}</span></div><p>{{ selected.ticket.description }}</p></article>
              <article v-for="message in selected.messages" :key="message.id" class="ticket-message" :class="message.sender_type === 'ADMIN' ? 'ticket-message-support' : 'ticket-message-user'"><div class="ticket-message-meta"><strong>{{ t(message.sender_type === 'ADMIN' ? 'support.team' : 'support.user') }}</strong><time :datetime="message.created_at">{{ formatDate(message.created_at) }}</time></div><p>{{ message.body }}</p></article>
            </div>
            <form v-if="selected.ticket.status !== 'CLOSED'" class="ticket-reply-form" @submit.prevent="sendReply"><label for="admin-ticket-reply">{{ t('support.reply') }}</label><textarea id="admin-ticket-reply" v-model="reply" :disabled="busy" rows="3" required :placeholder="t('support.replyPlaceholder')" /><div><span>{{ t('support.adminReplyHint') }}</span><button type="submit" class="ticket-button ticket-button-primary" :disabled="busy || !reply.trim()">{{ t(sending ? 'support.sending' : 'support.reply') }}<Icon name="chevronRight" size="sm" /></button></div></form>
            <div v-else class="ticket-closed-note"><Icon name="check" size="sm" />{{ t('support.adminClosedHint') }}</div>
          </template>
          <div v-else class="ticket-detail-empty"><span class="ticket-empty-icon ticket-empty-icon-large"><Icon name="chat" size="xl" /></span><h2>{{ t(tickets.length ? 'support.selectTicket' : 'support.empty') }}</h2><p>{{ t(tickets.length ? 'support.adminSelectHint' : 'support.adminEmptyHint') }}</p></div>
        </section>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
/** @brief Administrator ticket inbox with inline conversation and independent status updates. */
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { supportTicketsAPI, type SupportTicketAdminView, type SupportTicketMessage, type SupportTicketStatus } from '@/api/supportTickets'
import { useAppStore } from '@/stores/app'
import { extractI18nErrorMessage } from '@/utils/apiError'

/** @brief A ticket, its owner and ordered conversation returned by the admin detail endpoint. */
type AdminTicketDetail = SupportTicketAdminView & { messages: SupportTicketMessage[] }

const { t, locale } = useI18n()
const appStore = useAppStore()
const tickets = ref<SupportTicketAdminView[]>([])
const selected = ref<AdminTicketDetail | null>(null)
const activeTicketId = ref<number | null>(null)
const loading = ref(false)
const detailLoading = ref(false)
const listError = ref(false)
const detailError = ref(false)
const sending = ref(false)
const savingStatus = ref(false)
const busy = computed(() => sending.value || savingStatus.value)
const reply = ref('')
const nextStatus = ref<SupportTicketStatus>('OPEN')
const statuses: SupportTicketStatus[] = ['OPEN', 'RESOLVED', 'CLOSED']
const query = ref('')
const statusFilter = ref<SupportTicketStatus | 'ALL'>('ALL')
const expandedGroups = reactive<Record<SupportTicketStatus, boolean>>({ OPEN: true, RESOLVED: false, CLOSED: false })
const detailPanel = ref<HTMLElement | null>(null)
const conversation = ref<HTMLElement | null>(null)
let detailRequest = 0
let historyRevision = 0

/** @brief Search ticket and owner fields, then group matching items by their actual status. */
const ticketGroups = computed(() => {
  const search = query.value.trim().toLocaleLowerCase()
  const matches = tickets.value.filter(({ ticket, user }) => (statusFilter.value === 'ALL' || ticket.status === statusFilter.value) && (!search || `${ticket.id} ${ticket.subject} ${ticket.description} ${user.username} ${user.email}`.toLocaleLowerCase().includes(search)))
  return statuses.map(status => ({ status, items: matches.filter(item => item.ticket.status === status) })).filter(group => group.items.length > 0)
})

// Reveal explicit search/filter results; clearing them restores the default collapsed history.
watch([query, statusFilter], () => {
  for (const status of statuses) expandedGroups[status] = !!query.value.trim() || statusFilter.value === status || status === 'OPEN'
})

/** @brief Format conversation and history timestamps in the selected interface language. */
function formatDate(value: string, compact = false): string {
  return new Date(value).toLocaleString(locale.value, compact ? { month: 'short', day: 'numeric' } : { year: 'numeric', month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
}

/** @brief Display the existing translated API error feedback. */
function reportError(error: unknown) {
  appStore.showError(extractI18nErrorMessage(error, t, 'support.errors', t('common.error')))
}

/** @brief Load all tickets without replacing mutations confirmed during a slow list request. */
async function load() {
  if (loading.value) return
  loading.value = true
  listError.value = false
  const revision = historyRevision
  try {
    const { data } = await supportTicketsAPI.adminList()
    if (revision === historyRevision) tickets.value = data ?? []
  } catch (error) {
    if (revision === historyRevision) { listError.value = true; reportError(error) }
  } finally { loading.value = false }
}

/** @brief Restore the complete history and default group expansion. */
function resetFilters() { query.value = ''; statusFilter.value = 'ALL' }

/** @brief Update the matching history row while retaining the submitting user's identity. */
function updateHistory(detail: SupportTicketAdminView) {
  const item = { ticket: detail.ticket, user: detail.user }
  const index = tickets.value.findIndex(row => row.ticket.id === item.ticket.id)
  if (index < 0) tickets.value.unshift(item)
  else tickets.value[index] = item
}

/** @brief Select a conversation, ignoring late responses for previously selected tickets. */
async function openTicket(id: number, clearDraft = true) {
  if (busy.value) return
  const request = ++detailRequest
  activeTicketId.value = id
  selected.value = null
  detailLoading.value = true
  detailError.value = false
  if (clearDraft) reply.value = ''
  await nextTick()
  if (window.matchMedia('(max-width: 900px)').matches) detailPanel.value?.scrollIntoView({ block: 'start' })
  try {
    const { data } = await supportTicketsAPI.adminGet(id)
    if (request !== detailRequest) return
    selected.value = { ...data, messages: data.messages ?? [] }
    if (clearDraft) nextStatus.value = data.ticket.status
    updateHistory(data)
  } catch (error) {
    if (request !== detailRequest) return
    detailError.value = true
    reportError(error)
  } finally { if (request === detailRequest) detailLoading.value = false }
}

/** @brief Refresh both panes while preserving an unsent reply and pending status selection. */
async function refresh() {
  if (busy.value || loading.value) return
  const id = activeTicketId.value
  await Promise.all([load(), id !== null ? openTicket(id, false) : Promise.resolve()])
}

/** @brief Save only the selected status; a failed save leaves the current conversation and draft intact. */
async function saveStatus() {
  if (busy.value || !selected.value || nextStatus.value === selected.value.ticket.status) return
  const detail = selected.value
  savingStatus.value = true
  try {
    const { data } = await supportTicketsAPI.adminSetStatus(detail.ticket.id, nextStatus.value)
    historyRevision++
    detail.ticket = data
    nextStatus.value = data.status
    updateHistory(detail)
    if (data.status === 'OPEN') expandedGroups.OPEN = true
    appStore.showSuccess(t('support.statusSaved'))
  } catch (error) { reportError(error) }
  finally { savingStatus.value = false }
}

/** @brief Append an administrator reply exactly once; the API returns the ticket to OPEN. */
async function sendReply() {
  if (busy.value || !selected.value || selected.value.ticket.status === 'CLOSED' || !reply.value.trim()) return
  const detail = selected.value
  sending.value = true
  try {
    const { data } = await supportTicketsAPI.adminAddMessage(detail.ticket.id, reply.value.trim())
    historyRevision++
    detail.messages.push(data)
    detail.ticket = { ...detail.ticket, status: 'OPEN', updated_at: data.created_at }
    nextStatus.value = 'OPEN'
    expandedGroups.OPEN = true
    updateHistory(detail)
    reply.value = ''
    appStore.showSuccess(t('support.replySent'))
    await nextTick()
    if (conversation.value) conversation.value.scrollTop = conversation.value.scrollHeight
  } catch (error) { reportError(error) }
  finally { sending.value = false }
}

onMounted(load)
</script>

<style scoped src="@/styles/support-tickets.css"></style>
<style scoped>
.ticket-list-owner { display: flex; align-items: center; gap: 6px; color: var(--signal-muted); font-size: 11px; overflow-wrap: anywhere; }
.ticket-list-owner svg { flex-shrink: 0; }
.ticket-admin-controls { display: flex; align-items: center; justify-content: space-between; gap: 20px; padding: 18px 30px; border-bottom: 1px solid var(--signal-line); background: var(--signal-bg); }
.ticket-owner { display: flex; align-items: center; gap: 12px; min-width: 0; }
.ticket-owner .ticket-empty-icon { width: 36px; height: 36px; border-radius: 9px; flex-shrink: 0; }
.ticket-owner strong { display: block; font-size: 13px; font-weight: 500; overflow-wrap: anywhere; }
.ticket-owner > div > span { display: block; margin-top: 4px; color: var(--signal-muted); font-size: 11px; overflow-wrap: anywhere; }
.ticket-status-form { flex-shrink: 0; }
.ticket-status-form label { display: block; margin-bottom: 6px; color: var(--signal-muted); font-size: 11px; }
.ticket-status-form > div { display: flex; align-items: center; gap: 8px; }
.ticket-status-form .ticket-filter { max-width: 150px; }
@media (max-width: 1200px) { .ticket-admin-controls { align-items: flex-start; flex-direction: column; gap: 16px; } }
@media (max-width: 540px) {
  .ticket-admin-controls { padding: 18px 20px; }
  .ticket-status-form, .ticket-status-form > div { width: 100%; }
  .ticket-status-form .ticket-filter { flex: 1; max-width: none; }
}
</style>
