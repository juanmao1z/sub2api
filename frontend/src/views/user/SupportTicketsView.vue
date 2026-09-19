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
              <section v-for="group in ticketGroups" :key="group.status" class="ticket-group" :data-status="group.status">
                <h3>
                  <button :id="`ticket-group-heading-${group.status}`" type="button" class="ticket-group-toggle" :aria-expanded="expandedGroups[group.status]" :aria-controls="`ticket-group-items-${group.status}`" @click="expandedGroups[group.status] = !expandedGroups[group.status]">
                    <Icon name="chevronRight" size="sm" class="ticket-group-chevron" :class="{ 'ticket-group-chevron-open': expandedGroups[group.status] }" aria-hidden="true" />
                    <span class="ticket-dot" :class="`ticket-dot-${group.status.toLowerCase()}`"></span>
                    <span>{{ t(`support.status.${group.status}`) }}</span><span class="ticket-group-count">{{ group.items.length }}</span>
                  </button>
                </h3>
                <div v-show="expandedGroups[group.status]" :id="`ticket-group-items-${group.status}`" role="region" :aria-labelledby="`ticket-group-heading-${group.status}`">
                  <button v-for="ticket in group.items" :key="ticket.id" type="button" class="ticket-list-item" :class="{ 'ticket-list-item-active': !showCreate && activeTicketId === ticket.id }" :aria-pressed="!showCreate && activeTicketId === ticket.id" :disabled="busy" @click="openTicket(ticket.id)">
                    <span class="ticket-list-meta"><span>#{{ ticket.id }} · {{ t(ticket.type === 'REFUND' ? 'support.refund' : 'support.suggestion') }}</span><time :datetime="ticket.updated_at">{{ formatDate(ticket.updated_at, true) }}</time></span>
                    <span class="ticket-list-subject">{{ ticket.subject }}</span>
                    <span class="ticket-list-description">{{ ticket.description }}</span>
                    <span class="ticket-status"><span class="ticket-dot" :class="`ticket-dot-${ticket.status.toLowerCase()}`"></span>{{ t(`support.status.${ticket.status}`) }}</span>
                  </button>
                </div>
              </section>
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
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue'
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
const expandedGroups = reactive<Record<SupportTicketStatus, boolean>>({ OPEN: true, RESOLVED: false, CLOSED: false })
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

/** @brief Group filtered history by lifecycle, preserving the server's order inside each group. */
const ticketGroups = computed(() => statuses
  .map(status => ({ status, items: filteredTickets.value.filter(ticket => ticket.status === status) }))
  .filter(group => group.items.length > 0))

// Explicit searches and filters reveal their results; clearing them restores the default groups.
watch([query, statusFilter], () => {
  for (const status of statuses) {
    expandedGroups[status] = !!query.value.trim() || statusFilter.value === status || status === 'OPEN'
  }
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
    expandedGroups.OPEN = true
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

<style scoped src="@/styles/support-tickets.css"></style>
