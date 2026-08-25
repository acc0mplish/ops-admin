const TOKEN_KEY = 'ops-admin-token'
const USER_KEY = 'ops-admin-user'
const MENU_KEY = 'ops-admin-menus'
const PERMISSION_KEY = 'ops-admin-permissions'
const SIDEBAR_COLLAPSED_KEY = 'ops-admin-sidebar-collapsed'
const TOKEN_EXPIRES_AT_KEY = 'ops-admin-token-expires-at'
const LAST_ACTIVITY_AT_KEY = 'ops-admin-last-activity-at'

export function getToken() {
  return localStorage.getItem(TOKEN_KEY) || ''
}

export function setToken(token, expiresAt = 0) {
  localStorage.setItem(TOKEN_KEY, token)
  if (expiresAt) localStorage.setItem(TOKEN_EXPIRES_AT_KEY, String(expiresAt))
  if (!getLastActivityAt()) setLastActivityAt(Date.now())
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(TOKEN_EXPIRES_AT_KEY)
}

export function getTokenExpiresAt() {
  const stored = Number(localStorage.getItem(TOKEN_EXPIRES_AT_KEY) || 0)
  if (stored) return stored
  const token = getToken()
  if (!token) return 0
  try {
    const payload = JSON.parse(atob(token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')))
    return Number(payload.exp || 0) * 1000
  } catch {
    return 0
  }
}

export function getLastActivityAt() {
  return Number(localStorage.getItem(LAST_ACTIVITY_AT_KEY) || 0)
}

export function setLastActivityAt(timestamp) {
  localStorage.setItem(LAST_ACTIVITY_AT_KEY, String(timestamp))
}

export function setUser(user) {
  localStorage.setItem(USER_KEY, JSON.stringify(user || {}))
}

export function getUser() {
  const raw = localStorage.getItem(USER_KEY)
  return raw ? JSON.parse(raw) : {}
}

export function clearUser() {
  localStorage.removeItem(USER_KEY)
}

export function setMenus(menus) {
  localStorage.setItem(MENU_KEY, JSON.stringify(menus || []))
}

export function getMenus() {
  const raw = localStorage.getItem(MENU_KEY)
  return raw ? JSON.parse(raw) : []
}

export function clearMenus() {
  localStorage.removeItem(MENU_KEY)
}

export function setPermissions(permissions) {
  localStorage.setItem(PERMISSION_KEY, JSON.stringify(permissions || []))
}

export function getPermissions() {
  const raw = localStorage.getItem(PERMISSION_KEY)
  return raw ? JSON.parse(raw) : []
}

export function clearPermissions() {
  localStorage.removeItem(PERMISSION_KEY)
}

export function setSidebarCollapsed(collapsed) {
  localStorage.setItem(SIDEBAR_COLLAPSED_KEY, collapsed ? '1' : '0')
}

export function getSidebarCollapsed() {
  return localStorage.getItem(SIDEBAR_COLLAPSED_KEY) === '1'
}

export function logout() {
  clearToken()
  clearUser()
  clearMenus()
  clearPermissions()
  localStorage.removeItem(LAST_ACTIVITY_AT_KEY)
}
