const USER_DASHBOARD = '/dashboard'
const ADMIN_DASHBOARD = '/admin/dashboard'

export function resolvePostLoginRedirect(redirect: unknown, isAdmin: boolean): string {
  const fallback = isAdmin ? ADMIN_DASHBOARD : USER_DASHBOARD

  if (typeof redirect !== 'string' || !redirect.startsWith('/') || redirect.startsWith('//')) {
    return fallback
  }

  if (isAdmin && redirect === USER_DASHBOARD) {
    return ADMIN_DASHBOARD
  }

  if (!isAdmin && redirect.startsWith('/admin')) {
    return USER_DASHBOARD
  }

  return redirect
}
