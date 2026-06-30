export function maskLeaderboardEmail(email: string): string {
  if (!email) return '***'

  const [local, domain] = email.split('@')
  if (!domain) return email

  if (local.length <= 2) return email

  return `${local[0]}***${local[local.length - 1]}@${domain}`
}
