<template>
  <AppLayout>
    <div class="ticket-center">
      <header class="ticket-toolbar">
        <div>
          <h1>{{ t('support.centerTitle') }}</h1>
          <p>{{ t('support.centerDescription') }}</p>
        </div>
        <div class="ticket-tools">
          <label class="ticket-search">
            <Icon name="search" size="sm" aria-hidden="true" />
            <input v-model="query" type="search" :aria-label="t('support.search')" :placeholder="t('support.search')" />
          </label>
          <select v-model="statusFilter" class="ticket-filter" :aria-label="t('support.filterStatus')">
            <option value="ALL">{{ t('support.allTickets') }}</option>
            <option v-for="status in statuses" :key="status" :value="status">{{ t(`support.status.${status}`) }}</option>
          </select>
          <button type="button" class="ticket-icon-button" :aria-label="t('common.refresh')" :title="t('common.refresh')" :disabled="loading || busy" @click="refresh">
            <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
          </button>
          <button type="button" class="ticket-button ticket-button-primary" :disabled="busy" @click="startCreate">
            <Icon name="plus" size="sm" />{{ t('support.newTicket') }}
          </button>
        </div>
      </header>

      <div class="ticket-workspace">
        <aside class="ticket-history" :aria-label="t('support.title')">
          <div class="ticket-history-heading"><h2>{{ t('support.title') }}</h2><span class="ticket-count">{{ t('support.ticketCount', { count: tickets.length }) }}</span></div>
          <div class="ticket-status-filters">
            <button v-for="status in statuses" :key="status" type="button" :aria-pressed="statusFilter === status" @click="statusFilter = statusFilter === status ? 'ALL' : status">
              <span class="ticket-dot" :class="`ticket-dot-${status.toLowerCase()}`"></span>
              {{ t(`support.status.${status}`) }}<span>{{ tickets.filter(ticket => ticket.status === status).length }}</span>
            </button>
          </div>
          <div class="ticket-list" :aria-busy="loading">
            <div v-if="loading && !tickets.length" class="ticket-list-empty" role="status">{{ t('common.loading') }}</div>
            <div v-else-if="listError" class="ticket-list-empty" role="alert">
              <Icon name="refresh" size="lg" /><p>{{ t('support.loadFailed') }}</p>
              <button class="ticket-button" type="button" @click="load">{{ t('support.retry') }}</button>
            </div>
            <template v-else-if="filteredTickets.length">
              <button v-for="ticket in filteredTickets" :key="ticket.id" type="button" class="ticket-list-item" :class="{ 'ticket-list-item-active': !showCreate && activeTicketId === ticket.id }" :aria-pressed="!showCreate && activeTicketId === ticket.id" :disabled="busy" @click="openTicket(ticket.id)">
                <span class="ticket-list-meta"><span>#{{ ticket.id }} · {{ t(ticket.type === 'REFUND' ? 'support.refund' : 'support.suggestion') }}</span><time :datetime="ticket.updated_at">{{ formatDate(ticket.updated_at, true) }}</time></span>
                <span class="ticket-list-subject">{{ ticket.subject }}</span>
                <span class="ticket-list-description">{{ ticket.description }}</span>
                <span class="ticket-status"><span class="ticket-dot" :class="`ticket-dot-${ticket.status.toLowerCase()}`"></span>{{ t(`support.status.${ticket.status}`) }}</span>
              </button>
            </template>
            <div v-else class="ticket-list-empty">
              <span class="ticket-empty-icon"><Icon :name="tickets.length ? 'search' : 'chat'" size="lg" /></span>
              <h3>{{ t(tickets.length ? 'support.noMatches' : 'support.empty') }}</h3>
              <p>{{ t(tickets.length ? 'support.changeFilters' : 'support.historyHint') }}</p>
              <button v-if="tickets.length" type="button" class="ticket-button" @click="resetFilters">{{ t('support.resetFilters') }}</button>
            </div>
          </div>
        </aside>

        <section ref="detailPanel" class="ticket-detail" :aria-label="t(showCreate ? 'support.newTicket' : 'support.details')" :aria-busy="detailLoading">
          <template v-if="showCreate">
            <header class="ticket-panel-heading">
              <div><span class="ticket-eyebrow">{{ t('support.contactSupport') }}</span><h2>{{ t('support.newTicket') }}</h2><p>{{ t('support.createHint') }}</p></div>
              <button type="button" class="ticket-icon-button" :aria-label="t('common.cancel')" :disabled="busy" @click="showCreate = false"><Icon name="x" size="sm" /></button>
            </header>
            <div class="ticket-form-scroll">
              <form class="ticket-create-form" @submit.prevent="createTicket">
                <fieldset :disabled="busy" class="ticket-fields">
                  <fieldset class="ticket-type-field">
                    <legend>{{ t('support.type') }}</legend>
                    <div class="ticket-type-options">
                      <label v-for="type in ticketTypes" :key="type" class="ticket-type-option" :class="{ 'ticket-type-option-active': form.type === type }">
                        <input v-model="form.type" type="radio" name="ticket-type" :value="type" />
                        <Icon :name="type === 'REFUND' ? 'creditCard' : 'chat'" size="md" />
                        <span><strong>{{ t(type === 'REFUND' ? 'support.refund' : 'support.suggestion') }}</strong><small>{{ t(type === 'REFUND' ? 'support.refundHint' : 'support.suggestionHint') }}</small></span>
                      </label>
                    </div>
                  </fieldset>
                  <div class="ticket-field"><label for="ticket-subject">{{ t('support.subject') }}</label><input id="ticket-subject" ref="subjectInput" v-model="form.subject" required :placeholder="t('support.subjectPlaceholder')" /></div>
                  <div v-if="form.type === 'REFUND'" class="ticket-refund-fields">
                    <div class="ticket-field"><label for="ticket-order">{{ t('support.orderId') }}</label><input id="ticket-order" v-model.number="form.order_id" type="number" min="1" step="1" required :placeholder="t('support.orderPlaceholder')" /></div>
                    <div class="ticket-field"><label for="ticket-contact">{{ t('support.contact') }}</label><input id="ticket-contact" v-model="form.contact" required :placeholder="t('support.contactPlaceholder')" /></div>
                  </div>
                  <div class="ticket-field"><label for="ticket-description">{{ t('support.description') }}</label><textarea id="ticket-description" v-model="form.description" rows="7" required :placeholder="t('support.descriptionPlaceholder')" /></div>
                  <p class="ticket-form-note"><Icon name="lock" size="sm" />{{ t('support.privacyHint') }}</p>
                </fieldset>
                <div class="ticket-form-actions"><button type="button" class="ticket-button" :disabled="busy" @click="showCreate = false">{{ t('common.cancel') }}</button><button type="submit" class="ticket-button ticket-button-primary" :disabled="busy || !canCreate">{{ t(submitting ? 'support.submitting' : 'support.submitTicket') }}<Icon name="chevronRight" size="sm" /></button></div>
              </form>
            </div>
          </template>
          <div v-else-if="detailLoading" class="ticket-detail-empty" role="status"><Icon name="refresh" size="lg" class="animate-spin" /><p>{{ t('common.loading') }}</p></div>
          <div v-else-if="detailError" class="ticket-detail-empty" role="alert"><span class="ticket-empty-icon"><Icon name="chat" size="lg" /></span><h2>{{ t('support.loadFailed') }}</h2><button type="button" class="ticket-button" @click="activeTicketId !== null && openTicket(activeTicketId)">{{ t('support.retry') }}</button></div>
          <template v-else-if="selected">
            <header class="ticket-panel-heading">
              <div><div class="ticket-detail-meta"><span>#{{ selected.ticket.id }} · {{ t(selected.ticket.type === 'REFUND' ? 'support.refund' : 'support.suggestion') }}</span><span class="ticket-status"><span class="ticket-dot" :class="`ticket-dot-${selected.ticket.status.toLowerCase()}`"></span>{{ t(`support.status.${selected.ticket.status}`) }}</span></div><h2>{{ selected.ticket.subject }}</h2><p>{{ formatDate(selected.ticket.created_at) }}</p></div>
              <button v-if="selected.ticket.status !== 'CLOSED'" type="button" class="ticket-button" :disabled="busy" @click="closeTicket"><Icon name="check" size="sm" />{{ t('support.close') }}</button>
            </header>
            <div class="ticket-conversation">
              <div v-if="selected.ticket.order_id || selected.ticket.contact" class="ticket-order-info"><span v-if="selected.ticket.order_id">{{ t('support.orderId') }} · {{ selected.ticket.order_id }}</span><span v-if="selected.ticket.contact">{{ t('support.contact') }} · {{ selected.ticket.contact }}</span></div>
              <article class="ticket-message ticket-message-user"><div class="ticket-message-meta"><strong>{{ t('support.you') }}</strong><time :datetime="selected.ticket.created_at">{{ formatDate(selected.ticket.created_at) }}</time><span>{{ t('support.originalMessage') }}</span></div><p>{{ selected.ticket.description }}</p></article>
              <article v-for="message in selected.messages" :key="message.id" class="ticket-message" :class="message.sender_type === 'ADMIN' ? 'ticket-message-support' : 'ticket-message-user'"><div class="ticket-message-meta"><strong>{{ t(message.sender_type === 'ADMIN' ? 'support.team' : 'support.you') }}</strong><time :datetime="message.created_at">{{ formatDate(message.created_at) }}</time></div><p>{{ message.body }}</p></article>
            </div>
            <form v-if="selected.ticket.status !== 'CLOSED'" class="ticket-reply-form" @submit.prevent="sendReply">
              <label for="ticket-reply">{{ t('support.reply') }}</label><textarea id="ticket-reply" v-model="reply" rows="3" :disabled="busy" :placeholder="t('support.replyPlaceholder')" required />
              <div><span>{{ t('support.privacyHint') }}</span><button type="submit" class="ticket-button ticket-button-primary" :disabled="busy || !reply.trim()">{{ t(replying ? 'support.sending' : 'support.reply') }}<Icon name="chevronRight" size="sm" /></button></div>
            </form>
            <div v-else class="ticket-closed-note"><Icon name="check" size="sm" />{{ t('support.closedHint') }}</div>
          </template>
          <div v-else class="ticket-detail-empty">
            <span class="ticket-empty-icon ticket-empty-icon-large"><Icon name="chat" size="xl" /></span>
            <h2>{{ t(tickets.length ? 'support.selectTicket' : 'support.empty') }}</h2>
            <p>{{ t(tickets.length ? 'support.selectHint' : 'support.emptyHint') }}</p>
            <button type="button" class="ticket-button ticket-button-primary" @click="startCreate"><Icon name="plus" size="sm" />{{ t('support.newTicket') }}</button>
          </div>
        </section>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
/** @brief User ticket workspace with persistent history and an inline composer. */
import { computed, nextTick, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { supportTicketsAPI, type SupportTicket, type SupportTicketDetail, type SupportTicketStatus, type SupportTicketType } from '@/api/supportTickets'
import { useAppStore } from '@/stores/app'
import { extractI18nErrorMessage } from '@/utils/apiError'

const { t, locale } = useI18n()
const appStore = useAppStore()
const tickets = ref<SupportTicket[]>([])
const selected = ref<SupportTicketDetail | null>(null)
const activeTicketId = ref<number | null>(null)
const loading = ref(false)
const detailLoading = ref(false)
const listError = ref(false)
const detailError = ref(false)
const submitting = ref(false)
const replying = ref(false)
const closing = ref(false)
const showCreate = ref(false)
const query = ref('')
const statusFilter = ref<SupportTicketStatus | 'ALL'>('ALL')
const reply = ref('')
const detailPanel = ref<HTMLElement | null>(null)
const subjectInput = ref<HTMLInputElement | null>(null)
const statuses: SupportTicketStatus[] = ['OPEN', 'RESOLVED', 'CLOSED']
const ticketTypes: SupportTicketType[] = ['SUGGESTION', 'REFUND']
const form = reactive<{ type: SupportTicketType; subject: string; description: string; contact: string; order_id?: number | '' }>({ type: 'SUGGESTION', subject: '', description: '', contact: '' })
let detailRequest = 0
let historyRevision = 0
const busy = computed(() => submitting.value || replying.value || closing.value)
const canCreate = computed(() => !!form.subject.trim() && !!form.description.trim() && (form.type !== 'REFUND' || (typeof form.order_id === 'number' && Number.isSafeInteger(form.order_id) && form.order_id > 0 && !!form.contact.trim())))
const filteredTickets = computed(() => {
  const search = query.value.trim().toLocaleLowerCase()
  return tickets.value.filter(ticket => (statusFilter.value === 'ALL' || ticket.status === statusFilter.value) && (!search || `${ticket.id} ${ticket.subject} ${ticket.description}`.toLocaleLowerCase().includes(search)))
})

/** @brief Format ticket timestamps using the current interface language. */
function formatDate(value: string, compact = false): string {
  return new Date(value).toLocaleString(locale.value, compact ? { month: 'short', day: 'numeric' } : { year: 'numeric', month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
}

/** @brief Present API failures through the existing translated error handler. */
function reportError(error: unknown) {
  appStore.showError(extractI18nErrorMessage(error, t, 'support.errors', t('common.error')))
}

/** @brief Load owned tickets while distinguishing a failed request from an empty inbox. */
async function load() {
  if (loading.value) return
  loading.value = true
  listError.value = false
  const revision = historyRevision
  try {
    const { data } = await supportTicketsAPI.list()
    if (revision === historyRevision) tickets.value = data ?? []
  } catch (error) {
    if (revision === historyRevision) { listError.value = true; reportError(error) }
  }
  finally { loading.value = false }
}

/** @brief Restore the unfiltered history after a new ticket is created. */
function resetFilters() { query.value = ''; statusFilter.value = 'ALL' }

/** @brief Keep confirmed ticket changes visible in the history immediately. */
function updateHistory(ticket: SupportTicket) {
  const index = tickets.value.findIndex(item => item.id === ticket.id)
  if (index < 0) tickets.value.unshift(ticket)
  else tickets.value[index] = ticket
}

/** @brief Bring the right panel into view on narrow screens without scrolling desktop history. */
async function revealPanel() {
  await nextTick()
  if (window.matchMedia('(max-width: 900px)').matches) detailPanel.value?.scrollIntoView({ block: 'start' })
}

/** @brief Show the inline composer and retain its draft when visiting history. */
async function startCreate() {
  if (busy.value) return
  detailRequest++
  detailLoading.value = false
  detailError.value = false
  activeTicketId.value = selected.value?.ticket.id ?? null
  showCreate.value = true
  await revealPanel()
  subjectInput.value?.focus({ preventScroll: true })
}

/** @brief Load the selected conversation; ignore stale responses after switching tickets or composing. */
async function openTicket(id: number, clearReply = true) {
  if (busy.value) return
  const request = ++detailRequest
  activeTicketId.value = id
  showCreate.value = false
  selected.value = null
  detailError.value = false
  detailLoading.value = true
  if (clearReply) reply.value = ''
  void revealPanel()
  try {
    const { data } = await supportTicketsAPI.get(id)
    if (request !== detailRequest) return
    selected.value = { ticket: data.ticket, messages: data.messages ?? [] }
    updateHistory(data.ticket)
  } catch (error) {
    if (request !== detailRequest) return
    detailError.value = true
    reportError(error)
  } finally { if (request === detailRequest) detailLoading.value = false }
}

/** @brief Refresh the history and visible conversation without discarding a reply or creation draft. */
async function refresh() {
  if (busy.value || loading.value) return
  const id = !showCreate.value ? activeTicketId.value : null
  await Promise.all([load(), id !== null ? openTicket(id, false) : Promise.resolve()])
}

/** @brief Submit the draft once and select the returned ticket; retain input on failure. */
async function createTicket() {
  if (busy.value || !canCreate.value) return
  submitting.value = true
  try {
    const { data } = await supportTicketsAPI.create({
      type: form.type, subject: form.subject.trim(), description: form.description.trim(),
      ...(form.type === 'REFUND' ? { order_id: Number(form.order_id), contact: form.contact.trim() } : {}),
    })
    detailRequest++
    selected.value = { ticket: data, messages: [] }
    activeTicketId.value = data.id
    detailError.value = false
    showCreate.value = false
    reply.value = ''
    historyRevision++
    listError.value = false
    updateHistory(data)
    resetFilters()
    form.subject = ''; form.description = ''; form.contact = ''; form.order_id = undefined
    appStore.showSuccess(t('support.created'))
  } catch (error) { reportError(error) }
  finally { submitting.value = false }
}

/** @brief Append a confirmed reply once; closed tickets cannot accept new messages. */
async function sendReply() {
  if (busy.value || !selected.value || selected.value.ticket.status === 'CLOSED' || !reply.value.trim()) return
  const detail = selected.value
  replying.value = true
  try {
    const { data } = await supportTicketsAPI.addMessage(detail.ticket.id, reply.value.trim())
    historyRevision++
    detail.messages.push(data)
    detail.ticket = { ...detail.ticket, status: 'OPEN', updated_at: data.created_at }
    updateHistory(detail.ticket)
    reply.value = ''
    appStore.showSuccess(t('support.replySent'))
  } catch (error) { reportError(error) }
  finally { replying.value = false }
}

/** @brief Close the current ticket and update both panels from the server response. */
async function closeTicket() {
  if (busy.value || !selected.value || selected.value.ticket.status === 'CLOSED') return
  closing.value = true
  try {
    const { data } = await supportTicketsAPI.close(selected.value.ticket.id)
    historyRevision++
    selected.value.ticket = data
    updateHistory(data)
    appStore.showSuccess(t('support.closed'))
  } catch (error) { reportError(error) }
  finally { closing.value = false }
}

onMounted(load)
</script>

<style scoped>
/** @brief Neutral two-pane ticket layout shares the console's light and dark tokens. */
.ticket-center { color: var(--signal-text); }
.ticket-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 20px; margin-bottom: 22px; }
.ticket-toolbar h1 { font-size: 22px; font-weight: 600; letter-spacing: -.025em; }
.ticket-toolbar p { margin-top: 6px; color: var(--signal-muted); font-size: 13px; }
.ticket-tools { display: flex; align-items: center; gap: 10px; }
.ticket-search { display: flex; align-items: center; gap: 9px; width: 230px; padding: 0 12px; border: 1px solid var(--signal-line); border-radius: 8px; background: var(--signal-surface); color: var(--signal-muted); }
.ticket-search input { width: 100%; min-width: 0; height: 38px; border: 0; outline: none; background: transparent; color: var(--signal-text); font-size: 13px; }
.ticket-search:focus-within { outline: 2px solid var(--signal-muted); outline-offset: 2px; }
.ticket-filter { height: 40px; max-width: 170px; padding: 0 30px 0 12px; border: 1px solid var(--signal-line); border-radius: 8px; background-color: var(--signal-surface); color: var(--signal-text); font-size: 13px; }
.ticket-button, .ticket-icon-button { display: inline-flex; align-items: center; justify-content: center; gap: 8px; min-height: 38px; flex-shrink: 0; border: 1px solid var(--signal-line); border-radius: 7px; padding: 8px 14px; background: var(--signal-surface); color: var(--signal-text); font-size: 13px; font-weight: 500; line-height: 20px; transition: background-color 160ms, border-color 160ms; }
.ticket-icon-button { width: 38px; padding: 8px; }
.ticket-button:hover, .ticket-icon-button:hover { background: var(--signal-raised); border-color: var(--signal-control-line); }
.ticket-center .ticket-button-primary { border-color: var(--signal-text); background: var(--signal-text); color: var(--signal-surface); }
.ticket-center .ticket-button-primary:hover { background: var(--signal-accent); border-color: var(--signal-accent); }
.ticket-center button:disabled { cursor: not-allowed; opacity: .5; }
.ticket-center :is(button, select, input, textarea):focus-visible { outline: 2px solid var(--signal-muted); outline-offset: 3px; }
.ticket-workspace { display: grid; grid-template-columns: minmax(290px, 32%) minmax(0, 1fr); gap: 18px; height: calc(100dvh - 195px); min-height: 580px; }
.ticket-history, .ticket-detail { display: flex; flex-direction: column; min-width: 0; min-height: 0; overflow: hidden; border: 1px solid var(--signal-line); border-radius: 12px; background: var(--signal-surface); }
.ticket-history-heading { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 20px; }
.ticket-history-heading h2 { font-size: 14px; font-weight: 600; }
.ticket-count { padding: 3px 8px; border-radius: 5px; background: var(--signal-raised); color: var(--signal-muted); font-size: 11px; }
.ticket-status-filters { display: flex; flex-wrap: wrap; gap: 7px; padding: 0 20px 17px; border-bottom: 1px solid var(--signal-line); }
.ticket-status-filters button { display: inline-flex; align-items: center; gap: 6px; padding: 5px 7px; border: 1px solid transparent; border-radius: 5px; font-size: 11px; color: var(--signal-muted); transition: background-color 160ms; }
.ticket-status-filters button:hover, .ticket-status-filters button[aria-pressed='true'] { border-color: var(--signal-line); background: var(--signal-raised); color: var(--signal-text); }
.ticket-dot { width: 6px; height: 6px; flex-shrink: 0; border-radius: 50%; background: #92929a; }
.ticket-dot-open { background: #6281a1; }
.ticket-dot-resolved { background: #628b77; }
.ticket-list { flex: 1; min-height: 0; overflow-y: auto; padding: 8px; scrollbar-width: thin; }
.ticket-list-item { display: flex; width: 100%; flex-direction: column; gap: 9px; margin-bottom: 5px; padding: 16px; border: 1px solid transparent; border-radius: 8px; text-align: left; transition: background-color 160ms, border-color 160ms; }
.ticket-list-item:hover { background: var(--signal-bg); }
.ticket-list-item-active { border-color: var(--signal-line); background: var(--signal-raised); }
.ticket-list-meta { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 6px; color: var(--signal-muted); font-size: 11px; }
.ticket-list-subject { display: -webkit-box; overflow: hidden; -webkit-line-clamp: 2; -webkit-box-orient: vertical; font-size: 14px; font-weight: 600; overflow-wrap: anywhere; }
.ticket-list-description { overflow: hidden; white-space: nowrap; text-overflow: ellipsis; max-width: 100%; color: var(--signal-muted); font-size: 12px; }
.ticket-status { display: inline-flex; align-items: center; gap: 6px; color: var(--signal-muted); font-size: 11px; }
.ticket-list-empty, .ticket-detail-empty { display: flex; align-items: center; justify-content: center; flex-direction: column; gap: 14px; height: 100%; padding: 36px 24px; text-align: center; }
.ticket-list-empty { min-height: 240px; }
.ticket-empty-icon { display: inline-flex; align-items: center; justify-content: center; width: 44px; height: 44px; border: 1px solid var(--signal-line); border-radius: 12px; background: var(--signal-raised); color: var(--signal-muted); }
.ticket-empty-icon-large { width: 64px; height: 64px; margin-bottom: 8px; border-radius: 18px; }
.ticket-list-empty h3 { font-size: 14px; font-weight: 600; }
.ticket-detail-empty h2 { font-size: 20px; font-weight: 600; letter-spacing: -.02em; }
.ticket-list-empty p, .ticket-detail-empty p { max-width: 320px; color: var(--signal-muted); font-size: 13px; line-height: 1.9; }
.ticket-detail-empty .ticket-button { margin-top: 8px; }
.ticket-panel-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 20px; padding: 26px 30px; border-bottom: 1px solid var(--signal-line); }
.ticket-panel-heading > div { min-width: 0; }
.ticket-panel-heading h2 { font-size: 20px; line-height: 1.5; font-weight: 600; letter-spacing: -.025em; overflow-wrap: anywhere; }
.ticket-panel-heading p { margin-top: 8px; font-size: 12px; line-height: 1.8; color: var(--signal-muted); }
.ticket-eyebrow { display: block; margin-bottom: 7px; font-size: 11px; color: var(--signal-muted); }
.ticket-form-scroll { flex: 1; overflow-y: auto; scrollbar-width: thin; }
.ticket-create-form { max-width: 860px; margin: 0 auto; padding: 28px 30px; }
.ticket-fields { display: grid; gap: 24px; min-width: 0; }
.ticket-type-field { min-width: 0; }
.ticket-type-field legend, .ticket-field > label, .ticket-reply-form > label { display: block; margin-bottom: 10px; font-size: 13px; font-weight: 500; }
.ticket-type-options { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.ticket-type-option { position: relative; display: flex; align-items: center; gap: 12px; cursor: pointer; padding: 16px; border: 1px solid var(--signal-line); border-radius: 8px; transition: background-color 160ms, border-color 160ms; }
.ticket-type-option input { position: absolute; width: 1px; height: 1px; opacity: 0; }
.ticket-type-option:focus-within { outline: 2px solid var(--signal-muted); outline-offset: 3px; }
.ticket-type-option-active { background: var(--signal-raised); border-color: var(--signal-accent); }
.ticket-type-option strong { display: block; font-size: 13px; font-weight: 500; }
.ticket-type-option small { display: block; margin-top: 5px; color: var(--signal-muted); font-size: 11px; line-height: 1.6; }
.ticket-field input, .ticket-field textarea, .ticket-reply-form textarea { width: 100%; border: 1px solid var(--signal-line); border-radius: 7px; padding: 11px 13px; background: var(--signal-surface); color: var(--signal-text); font-size: 13px; line-height: 1.7; }
.ticket-center textarea { resize: vertical; min-height: 80px; }
.ticket-center :is(input, textarea)::placeholder { color: var(--signal-muted); opacity: .8; }
.ticket-refund-fields { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
.ticket-form-note { display: flex; align-items: center; gap: 7px; color: var(--signal-muted); font-size: 11px; line-height: 1.7; }
.ticket-form-note svg { flex-shrink: 0; }
.ticket-form-actions { display: flex; justify-content: flex-end; gap: 10px; margin-top: 28px; padding-top: 22px; border-top: 1px solid var(--signal-line); }
.ticket-detail-meta { display: flex; flex-wrap: wrap; gap: 16px; margin-bottom: 10px; color: var(--signal-muted); font-size: 11px; }
.ticket-conversation { flex: 1; min-height: 0; overflow-y: auto; padding: 26px 30px; scrollbar-width: thin; }
.ticket-order-info { display: flex; flex-wrap: wrap; gap: 10px 20px; margin-bottom: 24px; padding: 12px 16px; border: 1px solid var(--signal-line); border-radius: 7px; color: var(--signal-muted); font-size: 12px; overflow-wrap: anywhere; }
.ticket-message { margin-bottom: 20px; padding: 18px 20px; border: 1px solid var(--signal-line); border-radius: 9px; }
.ticket-message-support { background: var(--signal-raised); }
.ticket-message-meta { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 12px; color: var(--signal-muted); font-size: 11px; }
.ticket-message-meta strong { color: var(--signal-text); font-weight: 600; font-size: 12px; }
.ticket-message p { margin-top: 12px; white-space: pre-wrap; overflow-wrap: anywhere; font-size: 13px; line-height: 1.9; }
.ticket-reply-form { padding: 20px 30px; border-top: 1px solid var(--signal-line); }
.ticket-reply-form > div { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-top: 12px; }
.ticket-reply-form > div > span { color: var(--signal-muted); font-size: 11px; }
.ticket-closed-note { display: flex; align-items: center; justify-content: center; gap: 8px; padding: 20px; border-top: 1px solid var(--signal-line); color: var(--signal-muted); font-size: 12px; }
@media (max-width: 1200px) {
  .ticket-toolbar { align-items: flex-start; flex-direction: column; }
  .ticket-tools { width: 100%; }
  .ticket-search { flex: 1; }
  .ticket-workspace { height: calc(100dvh - 245px); grid-template-columns: 300px minmax(0, 1fr); }
  .ticket-type-option { padding: 12px; }
}
@media (max-width: 900px) {
  .ticket-workspace { height: auto; min-height: 0; grid-template-columns: minmax(0, 1fr); }
  .ticket-history { max-height: 350px; }
  .ticket-detail { min-height: 460px; scroll-margin-top: 80px; }
  .ticket-detail-empty { min-height: 460px; }
  .ticket-list { max-height: 240px; }
  .ticket-conversation { max-height: 600px; }
  .ticket-tools { flex-wrap: wrap; }
  .ticket-search { min-width: 160px; }
  .ticket-filter { max-width: none; }
}
@media (max-width: 540px) {
  .ticket-toolbar { gap: 16px; }
  .ticket-tools { gap: 8px; }
  .ticket-search { flex-basis: 100%; }
  .ticket-filter { flex: 1; min-width: 0; }
  .ticket-panel-heading { padding: 20px; gap: 12px; }
  .ticket-panel-heading h2 { font-size: 18px; }
  .ticket-panel-heading .ticket-button { padding-inline: 10px; }
  .ticket-create-form, .ticket-conversation, .ticket-reply-form { padding: 20px; }
  .ticket-type-options, .ticket-refund-fields { grid-template-columns: minmax(0, 1fr); }
  .ticket-reply-form > div { align-items: flex-end; }
}
@media (prefers-reduced-motion: reduce) {
  .ticket-center *, .ticket-center *::before, .ticket-center *::after { transition: none !important; animation: none !important; }
}
</style>
