<template>
  <AppLayout>
    <div class="space-y-4">
      <div class="flex items-center justify-between"><h1 class="page-title">{{ t('support.title') }}</h1><button class="btn btn-primary" @click="showCreate = true">{{ t('support.newTicket') }}</button></div>
      <div v-if="loading" class="card p-6 text-center text-gray-500">{{ t('common.loading') }}</div>
      <div v-else-if="tickets.length === 0" class="card p-6 text-center text-gray-500">{{ t('support.empty') }}</div>
      <div v-for="ticket in tickets" :key="ticket.id" class="card cursor-pointer p-4" @click="openTicket(ticket.id)">
        <div class="flex items-center justify-between"><div><span class="mr-2 rounded bg-gray-100 px-2 py-1 text-xs dark:bg-dark-700">{{ ticket.type === 'REFUND' ? t('support.refund') : t('support.suggestion') }}</span><span class="font-medium">{{ ticket.subject }}</span></div><span class="mr-2 rounded px-2 py-1 text-xs" :class="ticket.status === 'OPEN' ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300' : ticket.status === 'CLOSED' ? 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300' : 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-300'">{{ ticket.status }}</span></div>
        <p class="mt-2 line-clamp-2 text-sm text-gray-500">{{ ticket.description }}</p>
      </div>
    </div>
    <BaseDialog :show="showCreate" :title="t('support.newTicket')" @close="showCreate = false">
      <form class="space-y-4" @submit.prevent="createTicket"><div><label class="input-label">{{ t('support.type') }}</label><select v-model="form.type" class="input"><option value="SUGGESTION">{{ t('support.suggestion') }}</option><option value="REFUND">{{ t('support.refund') }}</option></select></div><div><label class="input-label">{{ t('support.subject') }}</label><input v-model="form.subject" class="input" required /></div><div v-if="form.type === 'REFUND'"><label class="input-label">{{ t('support.orderId') }}</label><input v-model.number="form.order_id" class="input" type="number" required /></div><div v-if="form.type === 'REFUND'"><label class="input-label">{{ t('support.contact') }}</label><input v-model="form.contact" class="input" required :placeholder="t('support.contactPlaceholder')" /></div><div><label class="input-label">{{ t('support.description') }}</label><textarea v-model="form.description" class="input" rows="5" required /></div><div class="flex justify-end gap-2"><button type="button" class="btn btn-secondary" @click="showCreate = false">{{ t('common.cancel') }}</button><button class="btn btn-primary" :disabled="submitting">{{ t('common.submit') }}</button></div></form>
    </BaseDialog>
    <BaseDialog :show="!!selected" :title="selected?.ticket.subject || ''" width="wide" @close="selected = null"><div v-if="selected" class="space-y-4"><div v-for="message in selected.messages" :key="message.id" class="rounded-lg bg-gray-50 p-3 dark:bg-dark-800"><div class="mb-1 text-xs text-gray-500">{{ message.sender_type }} · {{ formatDate(message.created_at) }}</div><p class="whitespace-pre-wrap text-sm">{{ message.body }}</p></div><textarea v-model="reply" class="input w-full" rows="3" :placeholder="t('support.replyPlaceholder')" /><div class="flex justify-end gap-2"><button class="btn btn-secondary" @click="closeTicket">{{ t('support.close') }}</button><button class="btn btn-primary" :disabled="!reply.trim()" @click="sendReply">{{ t('support.reply') }}</button></div></div></BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { supportTicketsAPI, type SupportTicket, type SupportTicketDetail } from '@/api/supportTickets'
import { useAppStore } from '@/stores/app'
import { extractI18nErrorMessage } from '@/utils/apiError'
const { t } = useI18n(); const appStore = useAppStore(); const tickets = ref<SupportTicket[]>([]); const loading = ref(false); const submitting = ref(false); const showCreate = ref(false); const selected = ref<SupportTicketDetail | null>(null); const reply = ref(''); const form = reactive<{type:'REFUND'|'SUGGESTION';subject:string;description:string;contact:string;order_id?:number}>({ type:'SUGGESTION', subject:'', description:'', contact:'' })
const formatDate = (value: string) => new Date(value).toLocaleString()
async function load() { loading.value = true; try { tickets.value = (await supportTicketsAPI.list()).data } catch (e) { appStore.showError(extractI18nErrorMessage(e, t, 'support.errors', t('common.error'))) } finally { loading.value = false } }
async function createTicket() { submitting.value = true; try { await supportTicketsAPI.create({ ...form, ...(form.type === 'REFUND' && form.order_id ? { order_id: form.order_id } : {}) }); showCreate.value = false; form.subject=''; form.description=''; form.contact=''; form.order_id=undefined; await load() } catch (e) { appStore.showError(extractI18nErrorMessage(e, t, 'support.errors', t('common.error'))) } finally { submitting.value = false } }
async function openTicket(id:number) { try { selected.value = (await supportTicketsAPI.get(id)).data; reply.value='' } catch (e) { appStore.showError(extractI18nErrorMessage(e, t, 'support.errors', t('common.error'))) } }
async function sendReply() { if (!selected.value || !reply.value.trim()) return; try { await supportTicketsAPI.addMessage(selected.value.ticket.id, reply.value.trim()); selected.value = (await supportTicketsAPI.get(selected.value.ticket.id)).data; reply.value='' } catch (e) { appStore.showError(extractI18nErrorMessage(e, t, 'support.errors', t('common.error'))) } }
async function closeTicket() { if (!selected.value) return; try { await supportTicketsAPI.close(selected.value.ticket.id); selected.value = (await supportTicketsAPI.get(selected.value.ticket.id)).data; await load() } catch (e) { appStore.showError(extractI18nErrorMessage(e, t, 'support.errors', t('common.error'))) } }
onMounted(load)
</script>
