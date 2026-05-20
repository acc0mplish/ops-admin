const TOKEN_KEY = 'ops-admin-token'
const USER_KEY = 'ops-admin-user'
const MENU_KEY = 'ops-admin-menus'
const PERMISSION_KEY = 'ops-admin-permissions'
const SIDEBAR_COLLAPSED_KEY = 'ops-admin-sidebar-collapsed'

export function getToken() {
  return localStorage.getItem(TOKEN_KEY) || ''
}

export function setToken(token) {
  localStorage.setItem(TOKEN_KEY, token)
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY)
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
}
