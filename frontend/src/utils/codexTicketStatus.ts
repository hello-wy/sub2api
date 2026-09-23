import type { CodexAccountTicketStatus } from '@/api/admin/codexTickets'

export function ticketVerificationBlocked(status?: CodexAccountTicketStatus | null): boolean {
  return !!status && (status.authentication_blocked === true || status.credential_current === false || status.identity_current === false)
}

export function ticketVerificationStateKey(status?: CodexAccountTicketStatus | null): string {
  if (status?.authentication_blocked) return 'authenticationBlocked'
  if (ticketVerificationBlocked(status)) return 'needsVerification'
  return ''
}
