/** @file Admin ticket inbox regressions: grouping, inline replies, status changes and stale responses. */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import SupportTicketsView from '../SupportTicketsView.vue'
import common from '@/i18n/locales/en/common'
import type { SupportTicketAdminView, SupportTicketMessage, SupportTicketStatus } from '@/api/supportTickets'

const { api, showError, showSuccess } = vi.hoisted(() => ({
  api: { adminList: vi.fn(), adminGet: vi.fn(), adminAddMessage: vi.fn(), adminSetStatus: vi.fn() },
  showError: vi.fn(), showSuccess: vi.fn(),
}))
vi.mock('@/api/supportTickets', () => ({ supportTicketsAPI: api }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess }) }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<div><slot /></div>' } }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({
  locale: { value: 'en' },
  t: (key: string) => key.split('.').reduce<unknown>((value, part) => (value as Record<string, unknown>)?.[part], common) ?? key,
}) }))

/** @brief Provide an admin ticket fixture including owner and refund details. */
function row(id: number, status: SupportTicketStatus = 'OPEN'): SupportTicketAdminView {
  return {
    ticket: { id, user_id: id + 10, type: 'REFUND', status, subject: `Request ${id}`, description: `Original description ${id}`, order_id: 42, contact: 'contact@example.test', created_at: '2026-09-19T09:00:00Z', updated_at: '2026-09-19T10:00:00Z' },
    user: { id: id + 10, username: `User ${id}`, email: `user${id}@example.test` },
  }
}

/** @brief Hold an API response to test duplicate submissions and selection races. */
function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (error: Error) => void
  const promise = new Promise<T>((accept, fail) => { resolve = accept; reject = fail })
  return { promise, resolve, reject }
}

const wrappers: ReturnType<typeof mount>[] = []
/** @brief Mount the admin page against mocked APIs in a real document for visibility checks. */
async function mountPage() {
  const wrapper = mount(SupportTicketsView, { attachTo: document.body, global: { stubs: { Icon: true } } })
  wrappers.push(wrapper)
  await flushPromises()
  return wrapper
}

/** @brief Open the initially expanded active request. */
async function openFirst(wrapper: ReturnType<typeof mount>) {
  await wrapper.get('[data-status="OPEN"] .ticket-list-item').trigger('click')
  await flushPromises()
}

beforeEach(() => {
  vi.resetAllMocks()
  api.adminList.mockResolvedValue({ data: [row(1), row(2, 'RESOLVED'), row(3, 'CLOSED')] })
  api.adminGet.mockImplementation(async (id: number) => ({ data: { ...row(id, id === 3 ? 'CLOSED' : id === 2 ? 'RESOLVED' : 'OPEN'), messages: [] } }))
  vi.stubGlobal('matchMedia', () => ({ matches: false }))
})
afterEach(() => { wrappers.splice(0).forEach(wrapper => wrapper.unmount()); vi.unstubAllGlobals() })

describe('admin ticket inbox', () => {
  it('expands active tickets, collapses finished groups and shows owner and original issue inline', async () => {
    const wrapper = await mountPage()
    expect(wrapper.get('[data-status="OPEN"] .ticket-list-item').isVisible()).toBe(true)
    expect(wrapper.get('[data-status="RESOLVED"] .ticket-list-item').isVisible()).toBe(false)
    expect(wrapper.get('[data-status="CLOSED"] .ticket-list-item').isVisible()).toBe(false)
    await openFirst(wrapper)
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(wrapper.get('.ticket-owner').text()).toContain('user1@example.test')
    expect(wrapper.get('.ticket-conversation').text()).toContain('Original description 1')
    expect(wrapper.get('.ticket-order-info').text()).toContain('contact@example.test')
    expect(wrapper.get('.ticket-order-info').text()).toContain('42')
  })

  it('searches by owner email and restores collapsed groups after clearing filters', async () => {
    const wrapper = await mountPage()
    await wrapper.get('.ticket-search input').setValue('user2@example.test')
    expect(wrapper.findAll('.ticket-list-item')).toHaveLength(1)
    expect(wrapper.get('.ticket-list-item').isVisible()).toBe(true)
    await wrapper.get('.ticket-search input').setValue('')
    expect(wrapper.get('[data-status="RESOLVED"] .ticket-list-item').isVisible()).toBe(false)
    await wrapper.get('.ticket-filter').setValue('CLOSED')
    expect(wrapper.get('[data-status="CLOSED"] .ticket-list-item').isVisible()).toBe(true)
  })

  it('saves a status without sending the reply draft and moves the request into its group', async () => {
    api.adminSetStatus.mockResolvedValue({ data: row(1, 'RESOLVED').ticket })
    const wrapper = await mountPage()
    await openFirst(wrapper)
    await wrapper.get('#admin-ticket-reply').setValue('Unsent draft')
    await wrapper.get('#admin-ticket-status').setValue('RESOLVED')
    await wrapper.get('.ticket-status-form').trigger('submit')
    await flushPromises()
    expect(api.adminSetStatus).toHaveBeenCalledWith(1, 'RESOLVED')
    expect(api.adminAddMessage).not.toHaveBeenCalled()
    expect(wrapper.get<HTMLTextAreaElement>('#admin-ticket-reply').element.value).toBe('Unsent draft')
    expect(wrapper.get('[data-status="RESOLVED"] .ticket-group-count').text()).toBe('2')
    expect(wrapper.get('[data-status="RESOLVED"] .ticket-group-toggle').attributes('aria-expanded')).toBe('false')
    expect(wrapper.get('.ticket-panel-heading').text()).toContain('Request 1')
  })

  it('sends a reply only once, reopens resolved tickets and does not submit a separate status request', async () => {
    const pending = deferred<{ data: SupportTicketMessage }>()
    api.adminAddMessage.mockReturnValue(pending.promise)
    const wrapper = await mountPage()
    await wrapper.get('[data-status="RESOLVED"] .ticket-group-toggle').trigger('click')
    await wrapper.get('[data-status="RESOLVED"] .ticket-list-item').trigger('click')
    await flushPromises()
    await wrapper.get('#admin-ticket-reply').setValue(' Answer ')
    await wrapper.get('.ticket-reply-form').trigger('submit')
    await wrapper.get('.ticket-reply-form').trigger('submit')
    expect(api.adminAddMessage).toHaveBeenCalledTimes(1)
    expect(api.adminAddMessage).toHaveBeenCalledWith(2, 'Answer')
    expect(wrapper.get<HTMLButtonElement>('.ticket-list-item').element.disabled).toBe(true)
    pending.resolve({ data: { id: 99, ticket_id: 2, sender_id: 1, sender_type: 'ADMIN', body: 'Answer', created_at: '2026-09-19T11:00:00Z' } })
    await flushPromises()
    expect(api.adminSetStatus).not.toHaveBeenCalled()
    expect(wrapper.get('.ticket-message-support').text()).toContain('Answer')
    expect(wrapper.get<HTMLSelectElement>('#admin-ticket-status').element.value).toBe('OPEN')
    expect(wrapper.get('[data-status="OPEN"] .ticket-group-count').text()).toBe('2')
    expect(wrapper.get<HTMLTextAreaElement>('#admin-ticket-reply').element.value).toBe('')
  })

  it('requires reopening a closed ticket before replying', async () => {
    api.adminSetStatus.mockResolvedValue({ data: row(3).ticket })
    const wrapper = await mountPage()
    await wrapper.get('[data-status="CLOSED"] .ticket-group-toggle').trigger('click')
    await wrapper.get('[data-status="CLOSED"] .ticket-list-item').trigger('click')
    await flushPromises()
    expect(wrapper.find('.ticket-reply-form').exists()).toBe(false)
    await wrapper.get('#admin-ticket-status').setValue('OPEN')
    await wrapper.get('.ticket-status-form').trigger('submit')
    await flushPromises()
    expect(wrapper.find('.ticket-reply-form').exists()).toBe(true)
    expect(wrapper.get('[data-status="OPEN"] .ticket-group-count').text()).toBe('2')
  })

  it('retains drafts and the real ticket status when mutations fail', async () => {
    api.adminAddMessage.mockRejectedValue(new Error('offline'))
    api.adminSetStatus.mockRejectedValue(new Error('offline'))
    const wrapper = await mountPage()
    await openFirst(wrapper)
    await wrapper.get('#admin-ticket-reply').setValue('Keep this reply')
    await wrapper.get('.ticket-reply-form').trigger('submit')
    await flushPromises()
    await wrapper.get('#admin-ticket-status').setValue('CLOSED')
    await wrapper.get('.ticket-status-form').trigger('submit')
    await flushPromises()
    expect(wrapper.get<HTMLTextAreaElement>('#admin-ticket-reply').element.value).toBe('Keep this reply')
    expect(wrapper.get('.ticket-panel-heading .ticket-status').text()).toBe('In progress')
    expect(showError).toHaveBeenCalledTimes(2)
  })

  it('ignores old detail responses after another request is selected', async () => {
    const first = deferred<{ data: SupportTicketAdminView & { messages: SupportTicketMessage[] } }>()
    api.adminGet.mockReturnValueOnce(first.promise)
    const wrapper = await mountPage()
    await openFirst(wrapper)
    await wrapper.get('[data-status="RESOLVED"] .ticket-group-toggle').trigger('click')
    await wrapper.get('[data-status="RESOLVED"] .ticket-list-item').trigger('click')
    await flushPromises()
    first.resolve({ data: { ...row(1), messages: [] } })
    await flushPromises()
    expect(wrapper.get('.ticket-panel-heading').text()).toContain('Request 2')
  })

  it('preserves reply and status drafts on refresh and supports retrying failed list requests', async () => {
    api.adminList.mockRejectedValueOnce(new Error('offline'))
    const wrapper = await mountPage()
    expect(wrapper.get('.ticket-list [role="alert"]').text()).toContain('Unable to load')
    await wrapper.get('.ticket-list [role="alert"] button').trigger('click')
    await flushPromises()
    await openFirst(wrapper)
    await wrapper.get('#admin-ticket-reply').setValue('Draft')
    await wrapper.get('#admin-ticket-status').setValue('RESOLVED')
    await wrapper.get('.ticket-tools .ticket-icon-button').trigger('click')
    await flushPromises()
    expect(wrapper.get<HTMLTextAreaElement>('#admin-ticket-reply').element.value).toBe('Draft')
    expect(wrapper.get<HTMLSelectElement>('#admin-ticket-status').element.value).toBe('RESOLVED')
  })
})
