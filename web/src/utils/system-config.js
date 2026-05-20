const SYSTEM_CONFIG_KEY = 'ops-admin-system-config'

export function getSystemConfig() {
  const raw = localStorage.getItem(SYSTEM_CONFIG_KEY)
  return raw ? JSON.parse(raw) : {}
}

export function setSystemConfig(config) {
  localStorage.setItem(SYSTEM_CONFIG_KEY, JSON.stringify(config || {}))
}

export function clearSystemConfig() {
  localStorage.removeItem(SYSTEM_CONFIG_KEY)
}

export function applySystemTheme(config = {}) {
  const primaryColor = config.primaryColor || '#5b6cf9'
  document.documentElement.style.setProperty('--app-primary', primaryColor)
  document.documentElement.style.setProperty('--app-primary-soft', `${primaryColor}22`)
}

export function resolveSystemAsset(url) {
  if (!url) {
    return ''
  }
  if (/^https?:\/\//i.test(url) || url.startsWith('data:')) {
    return url
  }
  return url
}
