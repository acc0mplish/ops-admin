import { computed, ref } from 'vue'
import { t as koText, translateEntity as translateKoEntity, translateRoute as translateKoRoute } from './i18n'

const LOCALE_KEY = 'ops-admin-locale'
export const DEFAULT_LOCALE = 'ko-KR'
export const FALLBACK_LOCALE = 'en-US'
export const SUPPORTED_LOCALES = ['ko-KR', 'en-US']

const ko = {
  loginHeroSubtitle: '통합 운영 관리 플랫폼',
  passwordFieldsRequired: '비밀번호 정보를 모두 입력하십시오.',
  passwordMinLength: '새 비밀번호는 6자 이상이어야 합니다.',
  passwordMismatch: '새 비밀번호와 확인 비밀번호가 일치하지 않습니다.',
  department: '부서',
  position: '직책',
  status: '상태',
  active: '활성',
  inactive: '비활성',
  noPersonalNote: '등록된 개인 설명이 없습니다.',
  createdAt: '생성 일시',
  updatedAt: '수정 일시',
  administrator: '관리자',
  consoleScope: '콘솔 작업 범위',
  systemGovernance: '시스템 거버넌스',
  permissionConfiguration: '권한 구성',
  auditTrail: '감사 추적'
}

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
  noPost: 'No position assigned',
  consoleScope: 'Console scope',
  systemGovernance: 'System governance',
  permissionConfiguration: 'Permission configuration',
  auditTrail: 'Audit trail'
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
  let text
  if (locale.value === 'en-US') {
    text = en[key] || key
  } else {
    text = ko[key] || koText(key, params)
  }
  Object.keys(params).forEach((name) => {
    text = text.replaceAll(`{${name}}`, String(params[name]))
  })
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
