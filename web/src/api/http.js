import axios from 'axios'
import { ElMessage } from 'element-plus'
import router from '../router'
import {
  getLastActivityAt,
  getToken,
  getTokenExpiresAt,
  logout as clearLocalSession,
  setLastActivityAt,
  setToken
} from '../utils/auth'
import { ct } from '../utils/common-i18n'

const ACCESS_REFRESH_WINDOW = 5 * 60 * 1000
const SESSION_IDLE_TIMEOUT = 6 * 60 * 60 * 1000
const ACTIVITY_WRITE_INTERVAL = 15 * 1000

const http = axios.create({ baseURL: '', timeout: 15000 })
const authHttp = axios.create({ baseURL: '', timeout: 15000, withCredentials: true })

let refreshPromise = null
let lastActivityWrite = 0
let sessionExpiryMessageShown = false

function recordUserActivity() {
  const now = Date.now()
  if (now - lastActivityWrite < ACTIVITY_WRITE_INTERVAL) return
  const previousActivityAt = getLastActivityAt()
  if (getToken() && previousActivityAt && now - previousActivityAt >= SESSION_IDLE_TIMEOUT) {
    expireLocalSession(ct('idleSessionExpired'))
    return
  }
  lastActivityWrite = now
  setLastActivityAt(now)
}

if (typeof window !== 'undefined') {
  for (const eventName of ['pointerdown', 'keydown', 'touchstart', 'scroll']) {
    window.addEventListener(eventName, recordUserActivity, { passive: true })
  }
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'visible') recordUserActivity()
  })
}

function sessionIsIdle() {
  const lastActivityAt = getLastActivityAt()
  return Boolean(lastActivityAt && Date.now() - lastActivityAt >= SESSION_IDLE_TIMEOUT)
}

function expireLocalSession(message = ct('sessionExpired')) {
  clearLocalSession()
  if (router.currentRoute.value.path !== '/login') router.push('/login')
  if (!sessionExpiryMessageShown) {
    sessionExpiryMessageShown = true
    ElMessage.warning(message)
    window.setTimeout(() => { sessionExpiryMessageShown = false }, 2000)
  }
}

async function renewAccessToken(force = false) {
  const token = getToken()
  if (!token) return ''
  if (sessionIsIdle()) {
    expireLocalSession(ct('idleSessionExpired'))
    throw new Error('session idle timeout')
  }
  if (!force && getTokenExpiresAt() - Date.now() > ACCESS_REFRESH_WINDOW) return token
  if (!refreshPromise) {
    refreshPromise = authHttp.post('/api/v1/auth/refresh', {
      lastActivityAt: getLastActivityAt()
    }).then((response) => {
      const payload = response.data
      if (payload?.code !== 200 || !payload.data?.token) {
        throw new Error(payload?.message || 'token refresh failed')
      }
      setToken(payload.data.token, payload.data.accessTokenExpiresAt)
      return payload.data.token
    }).finally(() => {
      refreshPromise = null
    })
  }
  return refreshPromise
}

http.interceptors.request.use(async (config) => {
  let token = getToken()
  if (token && !config.skipAuthRefresh) token = await renewAccessToken()
  if (token) config.headers.Authorization = `Bearer ${token}`
  const lastActivityAt = getLastActivityAt()
  if (lastActivityAt) config.headers['X-User-Activity'] = String(lastActivityAt)
  return config
})

http.interceptors.response.use(
  (response) => {
    if (response.config.responseType === 'blob') return response
    const res = response.data
    if (res.code !== 200) {
      ElMessage.error(res.message || ct('requestFailed'))
      return Promise.reject(new Error(res.message || 'request failed'))
    }
    return res.data
  },
  async (error) => {
    const res = error.response?.data
    const config = error.config || {}
    if (res?.code === 401 && !config._authRetried && !config.skipAuthRefresh && getToken()) {
      config._authRetried = true
      try {
        const token = await renewAccessToken(true)
        config.headers = config.headers || {}
        config.headers.Authorization = `Bearer ${token}`
        return http(config)
      } catch {
        expireLocalSession()
        return Promise.reject(new Error(res?.message || ct('sessionExpired')))
      }
    }
    if (res?.code === 401) expireLocalSession(res.message)
    const message = res?.message || error.message || ct('networkError')
    if (res?.code !== 401) ElMessage.error(message)
    return Promise.reject(new Error(message))
  }
)

if (typeof window !== 'undefined') {
  window.setInterval(() => {
    if (getToken()) renewAccessToken().catch(() => expireLocalSession())
  }, 60 * 1000)
}

export default http
