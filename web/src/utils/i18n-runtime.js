import { computed, ref } from 'vue'
import { t as koText, translateEntity as translateKoEntity, translateRoute as translateKoRoute } from './i18n'

const LOCALE_KEY = 'ops-admin-locale'
export const DEFAULT_LOCALE = 'ko-KR'
export const FALLBACK_LOCALE = 'en-US'
export const SUPPORTED_LOCALES = ['ko-KR', 'en-US']

const en = {
  language: 'Korean',
  english: 'English',
  switchLanguage: 'Language',
  dashboard: 'Dashboard',
  profile: 'Profile',
  system: 'System Management',
  logs: 'Audit Logs',
  admin: 'User Management',
  role: 'Role Management',
  menu: 'Menu Management',
  dept: 'Department Management',
  post: 'Position Management',
  userCenter: 'User Center',
  logout: 'Log out',
  editProfile: 'Edit Profile',
  changePassword: 'Change Password',
  accountInfo: 'Account Information',
  extraInfo: 'Additional Information',
  loginWelcome: 'Sign in',
  loginHint: 'Enter your account and password to access the console.',
  loginSystem: 'Sign in',
  loginHeroSubtitle: 'Integrated operations management platform',
  username: 'Username',
  password: 'Password',
  nickname: 'Display Name',
  email: 'Email',
  phone: 'Phone',
  note: 'Note',
  confirmPassword: 'Confirm Password',
  currentPassword: 'Current Password',
  newPassword: 'New Password',
  cancel: 'Cancel',
  save: 'Save',
  confirmChange: 'Confirm Change',
  saveSuccess: 'Saved successfully.',
  profileUpdateSuccess: 'Profile updated successfully.',
  passwordUpdateSuccess: 'Password changed successfully.',
  passwordFieldsRequired: 'Enter all password fields.',
  passwordMinLength: 'The new password must be at least 6 characters.',
  passwordMismatch: 'The new password and confirmation do not match.',
  department: 'Department',
  position: 'Position',
  status: 'Status',
  active: 'Active',
  inactive: 'Inactive',
  noPersonalNote: 'No personal description has been registered.',
  createdAt: 'Created At',
  updatedAt: 'Updated At',
  administrator: 'Administrator',
  superAdmin: 'Super Administrator',
  headquarters: 'Headquarters',
  noDept: 'No department assigned',
  noPost: 'No position assigned'
}

const savedLocale = localStorage.getItem(LOCALE_KEY)
export const locale = ref(SUPPORTED_LOCALES.includes(savedLocale) ? savedLocale : DEFAULT_LOCALE)

if (!SUPPORTED_LOCALES.includes(savedLocale)) {
  localStorage.setItem(LOCALE_KEY, DEFAULT_LOCALE)
}

export function setLocale(value) {
  const next = SUPPORTED_LOCALES.includes(value) ? value : DEFAULT_LOCALE
  locale.value = next
  localStorage.setItem(LOCALE_KEY, next)
}

export function getLocale() {
  return locale.value
}

export function t(key, params = {}) {
  let text = locale.value === 'en-US' ? (en[key] || key) : koText(key, params)
  if (locale.value === 'en-US') {
    Object.keys(params).forEach((name) => {
      text = text.replaceAll(`{${name}}`, String(params[name]))
    })
  }
  return text
}

export function translateRoute(path, fallback) {
  if (locale.value === 'ko-KR') {
    return translateKoRoute(path, fallback)
  }
  return fallback || path || ''
}

export function translateEntity(value, fallback = '-') {
  if (locale.value === 'ko-KR') {
    return translateKoEntity(value, fallback)
  }
  const map = {
    'Super Admin': 'Super Administrator',
    'Super Administrator': 'Super Administrator',
    Headquarters: 'Headquarters'
  }
  return map[value] || value || fallback
}

export const currentLocale = computed(() => locale.value)
