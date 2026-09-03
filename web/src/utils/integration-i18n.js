import { currentLocale } from './i18n-runtime'

const ko = {
  toolRegistry: 'Tool Registry',
  toolRegistryDesc: 'AI가 호출할 수 있는 플랫폼 기능을 관리합니다. Read Tool은 자동 실행할 수 있고 모든 Kubernetes Write 작업은 반드시 수동 승인을 거칩니다.',
  enabledCount: '활성 / {total}',
  capabilityCount: '{count}개 기능',
  write: '변경',
  readOnly: '읽기 전용',
  debug: '디버그',
  cancel: '취소',
  executeReadOnly: 'Read Tool 실행',
  toolUpdated: 'Tool 설정을 업데이트했습니다.',
  toolResult: 'Tool 실행 결과',
  debugTool: 'Tool 디버그 · {name}',
  cloudCost: '클라우드 비용',
  assets: '자산',
  monitoring: '모니터링',
  grafana: 'Grafana',
  kubernetes: 'Kubernetes',
  conversations: 'AI 대화',
  publicNavigation: 'Public Navigation'
}

const en = {
  toolRegistry: 'Tool Registry',
  toolRegistryDesc: 'Manage platform capabilities callable by AI. Read tools may run automatically; every Kubernetes write operation requires explicit human confirmation.',
  enabledCount: 'Enabled / {total}',
  capabilityCount: '{count} capabilities',
  write: 'Write',
  readOnly: 'Read Only',
  debug: 'Debug',
  cancel: 'Cancel',
  executeReadOnly: 'Execute Read Tool',
  toolUpdated: 'Tool configuration updated.',
  toolResult: 'Tool Result',
  debugTool: 'Debug Tool · {name}',
  cloudCost: 'Cloud Cost',
  assets: 'Assets',
  monitoring: 'Monitoring',
  grafana: 'Grafana',
  kubernetes: 'Kubernetes',
  conversations: 'AI Conversations',
  publicNavigation: 'Public Navigation'
}

export function it(key, params = {}) {
  const dict = currentLocale.value === 'en-US' ? en : ko
  let text = dict[key] || en[key] || key
  Object.entries(params).forEach(([name, value]) => { text = text.replaceAll(`{${name}}`, String(value)) })
  return text
}

const legacyCategoryFragments = [
  ['\u4e91\u8d39\u7528', 'cloudCost'],
  ['\u8d44\u4ea7', 'assets'],
  ['\u76d1\u63a7', 'monitoring']
]

export function integrationCategoryLabel(value = '') {
  let result = String(value)
  for (const [legacy, key] of legacyCategoryFragments) result = result.replaceAll(legacy, it(key))
  return result
}

export function integrationCategoryKind(value = '') {
  const name = String(value)
  if (name.includes('\u4e91\u8d39\u7528') || name.includes('\u8d44\u4ea7') || /cloud|cost|asset/i.test(name)) return 'cost'
  if (name.includes('\u76d1\u63a7') || /monitor|metric|alert/i.test(name)) return 'monitor'
  if (/grafana/i.test(name)) return 'grafana'
  if (/kubernetes|k8s/i.test(name)) return 'kubernetes'
  return 'integration'
}
