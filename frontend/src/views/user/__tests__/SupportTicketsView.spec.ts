/** @file Regression coverage for the user ticket workspace and inline composer. */
import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import SupportTicketsView from '../SupportTicketsView.vue'
import common from '@/i18n/locales/en/common'
import type { SupportTicket, SupportTicketDetail } from '@/api/supportTickets'

const { api, showError, showSuccess } = vi.hoisted(() => ({
  api: { list: vi.fn(), get: vi.fn(), create: vi.fn(), addMessage: vi.fn(), close: vi.fn() },
  showError: vi.fn(), showSuccess: vi.fn(),
}))
vi.mock('vue-i18n', () => ({ useI18n: () => ({
  locale: { value: 'en' },
  t: (key: string) => key.split('.').reduce<unknown>((value, part) => (value as Record<string, unknown>)?.[part], common) ?? key,
}) }))
vi.mock('@/api/supportTickets', () => ({ supportTicketsAPI: api }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<div><slot /></div>' } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess }) }))

/** @brief Build an owned ticket fixture with a unique subject and original request. */
function ticket(id: number, status: SupportTicket['status'] = 'OPEN'): SupportTicket {
  return { id, user_id: 7, type: 'SUGGESTION', status, subject: `Question ${id}`, description: `Original issue ${id}`, created_at: '2026-09-19T09:00:00Z', updated_at: '2026-09-19T10:00:00Z' }
}

/** @brief Control completion order to exercise network races and pending mutations. */
function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (error: Error) => void
  const promise = new Promise<T>((accept, fail) => { resolve = accept; reject = fail })
  return { promise, resolve, reject }
}

const wrappers: ReturnType<typeof mount>[] = []
/** @brief Mount the real page with translated labels and stubbed network requests. */
async function mountPage() {
  const wrapper = mount(SupportTicketsView, { attachTo: document.body, global: {
    stubs: { Icon: true },
  } })
  wrappers.push(wrapper)
  await flushPromises()
  return wrapper
}

/** @brief Open the composer through its visible toolbar control. */
async function compose(wrapper: ReturnType<typeof mount>) {
  await wrapper.get('.ticket-tools .ticket-button-primary').trigger('click')
  await flushPromises()
}

/** @brief Fill the fields shared by suggestion and refund requests. */
async function fillDraft(wrapper: ReturnType<typeof mount>) {
  await wrapper.get('#ticket-subject').setValue(' A question ')
  await wrapper.get('#ticket-description').setValue(' Details about the problem ')
}

beforeEach(() => {
  vi.resetAllMocks()
  api.list.mockResolvedValue({ data: [ticket(1), ticket(2, 'RESOLVED'), ticket(3, 'CLOSED')] })
  api.get.mockImplementation(async (id: number) => ({ data: { ticket: ticket(id), messages: [] } }))
  vi.stubGlobal('matchMedia', () => ({ matches: false }))
})
afterEach(() => { wrappers.splice(0).forEach(wrapper => wrapper.unmount()); vi.unstubAllGlobals() })

describe('user ticket workspace', () => {
  it('expands only in-progress history by default and lets users toggle archived groups', async () => {
    const wrapper = await mountPage()
    expect(wrapper.get('[data-status="OPEN"] .ticket-group-toggle').attributes('aria-expanded')).toBe('true')
    for (const status of ['RESOLVED', 'CLOSED']) {
      const group = wrapper.get(`[data-status="${status}"]`)
      expect(group.get('.ticket-group-toggle').attributes('aria-expanded')).toBe('false')
      expect(group.get('.ticket-list-item').isVisible()).toBe(false)
      await group.get('.ticket-group-toggle').trigger('click')
      expect(group.get('.ticket-list-item').isVisible()).toBe(true)
      await group.get('.ticket-group-toggle').trigger('click')
      expect(group.get('.ticket-list-item').isVisible()).toBe(false)
    }
    expect(wrapper.get('[data-status="OPEN"] .ticket-list-item').isVisible()).toBe(true)
  })

  it('reveals search and status-filter matches and restores collapsed history when cleared', async () => {
    const wrapper = await mountPage()
    await wrapper.get('.ticket-filter').setValue('CLOSED')
    expect(wrapper.get('[data-status="CLOSED"] .ticket-list-item').isVisible()).toBe(true)
    await wrapper.get('.ticket-filter').setValue('ALL')
    expect(wrapper.get('[data-status="CLOSED"] .ticket-list-item').isVisible()).toBe(false)
    await wrapper.get('.ticket-search input').setValue('Question 2')
    expect(wrapper.get('[data-status="RESOLVED"] .ticket-list-item').isVisible()).toBe(true)
    await wrapper.get('.ticket-search input').setValue('')
    expect(wrapper.get('[data-status="RESOLVED"] .ticket-list-item').isVisible()).toBe(false)
  })

  it('moves a closed ticket into collapsed history while retaining its visible conversation', async () => {
    api.close.mockResolvedValue({ data: ticket(1, 'CLOSED') })
    const wrapper = await mountPage()
    await wrapper.get('[data-status="OPEN"] .ticket-list-item').trigger('click')
    await flushPromises()
    await wrapper.get('.ticket-panel-heading button').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-status="OPEN"]').exists()).toBe(false)
    expect(wrapper.get('[data-status="CLOSED"] .ticket-group-count').text()).toBe('2')
    expect(wrapper.get('[data-status="CLOSED"] .ticket-group-toggle').attributes('aria-expanded')).toBe('false')
    expect(wrapper.get('.ticket-panel-heading').text()).toContain('Question 1')
    expect(wrapper.get('.ticket-closed-note').isVisible()).toBe(true)
  })

  it('opens the inline composer from an empty inbox', async () => {
    api.list.mockResolvedValue({ data: [] })
    const wrapper = await mountPage()
    expect(wrapper.get('.ticket-list-empty').text()).toContain('No tickets yet')
    await wrapper.get('.ticket-detail-empty button').trigger('click')
    await flushPromises()
    expect(wrapper.find('.ticket-create-form').exists()).toBe(true)
    expect(wrapper.find('.ticket-detail-empty').exists()).toBe(false)
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
  })

  it('keeps a newly created ticket when the initial history response arrives late', async () => {
    const pending = deferred<{ data: SupportTicket[] }>()
    api.list.mockReturnValue(pending.promise)
    api.create.mockResolvedValue({ data: ticket(4) })
    const wrapper = await mountPage()
    await compose(wrapper)
    await fillDraft(wrapper)
    await wrapper.get('.ticket-create-form').trigger('submit')
    await flushPromises()
    pending.resolve({ data: [] })
    await flushPromises()
    expect(wrapper.findAll('.ticket-list-item')).toHaveLength(1)
    expect(wrapper.get('.ticket-list-item').text()).toContain('Question 4')
    expect(wrapper.get('.ticket-panel-heading').text()).toContain('Question 4')
  })

  it('replaces the right-hand empty state with an inline form while retaining history and drafts', async () => {
    const wrapper = await mountPage()
    expect(wrapper.get('.ticket-detail-empty').text()).toContain('Select a ticket')
    await compose(wrapper)
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(wrapper.find('.ticket-detail-empty').exists()).toBe(false)
    expect(wrapper.findAll('.ticket-list-item')).toHaveLength(3)
    await fillDraft(wrapper)
    await wrapper.findAll('.ticket-list-item')[0].trigger('click')
    await flushPromises()
    expect(wrapper.get('.ticket-conversation').text()).toContain('Original issue 1')
    await compose(wrapper)
    expect(wrapper.get<HTMLInputElement>('#ticket-subject').element.value).toBe(' A question ')
    expect(wrapper.get<HTMLTextAreaElement>('#ticket-description').element.value).toBe(' Details about the problem ')
  })

  it('filters real history by text and status and can clear an empty result', async () => {
    const wrapper = await mountPage()
    await wrapper.get('.ticket-search input').setValue('question 2')
    expect(wrapper.findAll('.ticket-list-item')).toHaveLength(1)
    await wrapper.get('.ticket-filter').setValue('OPEN')
    expect(wrapper.findAll('.ticket-list-item')).toHaveLength(0)
    expect(wrapper.get('.ticket-list-empty').text()).toContain('No matching tickets')
    await wrapper.get('.ticket-list-empty button').trigger('click')
    expect(wrapper.findAll('.ticket-list-item')).toHaveLength(3)
  })

  it('requires refund details, submits once, and selects the created ticket immediately', async () => {
    const pending = deferred<{ data: SupportTicket }>()
    api.create.mockReturnValue(pending.promise)
    const wrapper = await mountPage()
    await compose(wrapper)
    await fillDraft(wrapper)
    await wrapper.get('input[value="REFUND"]').setValue(true)
    expect(wrapper.get<HTMLButtonElement>('button[type="submit"]').element.disabled).toBe(true)
    await wrapper.get('#ticket-order').setValue('42')
    await wrapper.get('#ticket-contact').setValue(' help@example.test ')
    await wrapper.get('.ticket-create-form').trigger('submit')
    await wrapper.get('.ticket-create-form').trigger('submit')
    expect(api.create).toHaveBeenCalledTimes(1)
    expect(api.create).toHaveBeenCalledWith({ type: 'REFUND', subject: 'A question', description: 'Details about the problem', order_id: 42, contact: 'help@example.test' })
    expect(wrapper.get<HTMLButtonElement>('.ticket-tools .ticket-button-primary').element.disabled).toBe(true)
    pending.resolve({ data: { ...ticket(4), type: 'REFUND', order_id: 42, contact: 'help@example.test' } })
    await flushPromises()
    expect(wrapper.find('.ticket-create-form').exists()).toBe(false)
    expect(wrapper.get('.ticket-panel-heading').text()).toContain('Question 4')
    expect(wrapper.findAll('.ticket-list-item')).toHaveLength(4)
    expect(wrapper.get('.ticket-order-info').text()).toContain('42')
    expect(showSuccess).toHaveBeenCalledWith('Ticket submitted')
  })

  it('retains failed drafts and omits refund-only fields when submitting a suggestion', async () => {
    api.create.mockRejectedValue(new Error('offline'))
    const wrapper = await mountPage()
    await compose(wrapper)
    await fillDraft(wrapper)
    await wrapper.get('input[value="REFUND"]').setValue(true)
    await wrapper.get('#ticket-order').setValue('42')
    await wrapper.get('#ticket-contact').setValue('help@example.test')
    await wrapper.get('input[value="SUGGESTION"]').setValue(true)
    await wrapper.get('.ticket-create-form').trigger('submit')
    await flushPromises()
    expect(api.create).toHaveBeenCalledWith({ type: 'SUGGESTION', subject: 'A question', description: 'Details about the problem' })
    expect(wrapper.get<HTMLInputElement>('#ticket-subject').element.value).toBe(' A question ')
    expect(showError).toHaveBeenCalledOnce()
    expect(wrapper.get<HTMLButtonElement>('button[type="submit"]').element.disabled).toBe(false)
  })

  it('ignores stale details when opening another ticket or switching to the composer', async () => {
    const first = deferred<{ data: SupportTicketDetail }>()
    const second = deferred<{ data: SupportTicketDetail }>()
    api.get.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise)
    const wrapper = await mountPage()
    await wrapper.findAll('.ticket-list-item')[0].trigger('click')
    await wrapper.findAll('.ticket-list-item')[1].trigger('click')
    second.resolve({ data: { ticket: ticket(2), messages: [] } })
    await flushPromises()
    first.resolve({ data: { ticket: ticket(1), messages: [] } })
    await flushPromises()
    expect(wrapper.get('.ticket-panel-heading').text()).toContain('Question 2')
    const third = deferred<{ data: SupportTicketDetail }>()
    api.get.mockReturnValueOnce(third.promise)
    await wrapper.findAll('.ticket-list-item')[2].trigger('click')
    await compose(wrapper)
    third.reject(new Error('late failure'))
    await flushPromises()
    expect(wrapper.find('.ticket-create-form').exists()).toBe(true)
    expect(showError).not.toHaveBeenCalled()
  })

  it('distinguishes a failed history request from an empty history and allows retry', async () => {
    api.list.mockRejectedValueOnce(new Error('offline'))
    const wrapper = await mountPage()
    expect(wrapper.get('.ticket-list [role="alert"]').text()).toContain('Unable to load')
    await wrapper.get('.ticket-list [role="alert"] button').trigger('click')
    await flushPromises()
    expect(wrapper.findAll('.ticket-list-item')).toHaveLength(3)
  })

  it('renders the original issue, retains a failed reply, and updates both panels after closing', async () => {
    api.get.mockResolvedValue({ data: { ticket: ticket(1), messages: null } })
    api.addMessage.mockRejectedValueOnce(new Error('offline'))
    api.close.mockResolvedValue({ data: ticket(1, 'CLOSED') })
    const wrapper = await mountPage()
    await wrapper.findAll('.ticket-list-item')[0].trigger('click')
    await flushPromises()
    expect(wrapper.get('.ticket-conversation').text()).toContain('Original issue 1')
    await wrapper.get('#ticket-reply').setValue(' Reply details ')
    await wrapper.get('.ticket-reply-form').trigger('submit')
    await flushPromises()
    expect(wrapper.get<HTMLTextAreaElement>('#ticket-reply').element.value).toBe(' Reply details ')
    api.addMessage.mockResolvedValue({ data: { id: 50, ticket_id: 1, sender_id: 7, sender_type: 'USER', body: 'Reply details', created_at: '2026-09-19T11:00:00Z' } })
    await wrapper.get('.ticket-reply-form').trigger('submit')
    await flushPromises()
    expect(wrapper.get('.ticket-conversation').text()).toContain('Reply details')
    expect(wrapper.get<HTMLTextAreaElement>('#ticket-reply').element.value).toBe('')
    await wrapper.get('.ticket-panel-heading button').trigger('click')
    await flushPromises()
    expect(api.close).toHaveBeenCalledWith(1)
    expect(wrapper.find('.ticket-reply-form').exists()).toBe(false)
    expect(wrapper.get('.ticket-closed-note').text()).toContain('This ticket is closed')
    expect(wrapper.get('[data-status="CLOSED"]').text()).toContain('Question 1')
  })

  it('refreshes the selected conversation without clearing the reply draft', async () => {
    const wrapper = await mountPage()
    await wrapper.findAll('.ticket-list-item')[0].trigger('click')
    await flushPromises()
    await wrapper.get('#ticket-reply').setValue('Unsent reply')
    await wrapper.get('.ticket-tools .ticket-icon-button').trigger('click')
    await flushPromises()
    expect(wrapper.get<HTMLTextAreaElement>('#ticket-reply').element.value).toBe('Unsent reply')
  })
})
