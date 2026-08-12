import axios from 'axios'
import { ElMessage } from 'element-plus'
import router from '../router'
import { getToken, logout } from '../utils/auth'

const http = axios.create({
  baseURL: '',
  timeout: 15000
})

http.interceptors.request.use((config) => {
  const token = getToken()
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

http.interceptors.response.use(
  (response) => {
    if (response.config.responseType === 'blob') {
      return response
    }
    const res = response.data
    if (res.code !== 200) {
      if (res.code === 401) {
        logout()
        router.push('/login')
      }
      ElMessage.error(res.message || '请求失败')
      return Promise.reject(new Error(res.message || 'request failed'))
    }
    return res.data
  },
  (error) => {
    const res = error.response?.data
    if (res?.code === 401) {
      logout()
      router.push('/login')
    }
    const message = res?.message || error.message || '网络错误'
    ElMessage.error(message)
    return Promise.reject(new Error(message))
  }
)

export default http
