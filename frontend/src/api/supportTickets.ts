import { apiClient } from './client'

export type SupportTicketType = 'REFUND' | 'SUGGESTION'
export type SupportTicketStatus = 'OPEN' | 'RESOLVED' | 'CLOSED'
export type SupportTicketApprovalState = 'PENDING' | 'APPROVED' | 'REJECTED'

export interface SupportTicket {
  id: number
  user_id: number
  type: SupportTicketType
  status: SupportTicketStatus
  subject: string
  description: string
  contact?: string | null
  order_id?: number | null
  created_at: string
  updated_at: string
}

export interface SupportTicketMessage {
  id: number
  ticket_id: number
  sender_id: number
  sender_type: 'USER' | 'ADMIN'
  body: string
  created_at: string
}

export interface SupportTicketDetail {
  ticket: SupportTicket
  messages: SupportTicketMessage[]
}

export interface SupportTicketAdminView {
  ticket: SupportTicket
  user: { id: number; username: string; email: string }
}

export interface SupportTicketApproval {
  id: number
  ticket_link_id: number
  approval_state: SupportTicketApprovalState
  approved_by?: number | null
  approved_at?: string | null
  decision_reason?: string | null
  execution_result?: Record<string, unknown> | null
  created_at: string
  updated_at: string
}

export const supportTicketsAPI = {
  list() {
    return apiClient.get<SupportTicket[]>('/support/tickets')
  },
  create(data: { type: SupportTicketType; subject: string; description: string; contact?: string; order_id?: number }) {
    return apiClient.post<SupportTicket>('/support/tickets', data)
  },
  get(id: number) {
    return apiClient.get<SupportTicketDetail>(`/support/tickets/${id}`)
  },
  addMessage(id: number, body: string) {
    return apiClient.post<SupportTicketMessage>(`/support/tickets/${id}/messages`, { body })
  },
  close(id: number) {
    return apiClient.post<SupportTicket>(`/support/tickets/${id}/close`)
  },
  adminList() {
	return apiClient.get<SupportTicketAdminView[]>('/admin/support/tickets')
  },
  adminGet(id: number) {
	return apiClient.get<SupportTicketAdminView & { messages: SupportTicketMessage[] }>(`/admin/support/tickets/${id}`)
  },
  adminAddMessage(id: number, body: string) {
    return apiClient.post<SupportTicketMessage>(`/admin/support/tickets/${id}/messages`, { body })
  },
  adminSetStatus(id: number, status: SupportTicketStatus) {
    return apiClient.patch<SupportTicket>(`/admin/support/tickets/${id}/status`, { status })
  },
  adminGetApproval(id: number) {
    return apiClient.get<SupportTicketApproval>(`/admin/support/tickets/${id}/approval`)
  },
  adminDecideApproval(id: number, state: Exclude<SupportTicketApprovalState, 'PENDING'>, reason?: string) {
    return apiClient.patch<SupportTicketApproval>(`/admin/support/tickets/${id}/approval`, { state, reason })
  },
}
