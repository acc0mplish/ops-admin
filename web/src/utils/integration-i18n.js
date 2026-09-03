import { currentLocale } from './i18n-runtime'

const ko = {
  toolRegistry: 'Tool Registry', toolRegistryDesc: 'AI가 호출할 수 있는 플랫폼 기능을 관리합니다. Read Tool은 자동 실행할 수 있고 모든 Kubernetes Write 작업은 반드시 수동 승인을 거칩니다.', enabledCount: '활성 / {total}', capabilityCount: '{count}개 기능', write: '변경', readOnly: '읽기 전용', debug: '디버그', cancel: '취소', executeReadOnly: 'Read Tool 실행', toolUpdated: 'Tool 설정을 업데이트했습니다.', toolResult: 'Tool 실행 결과', debugTool: 'Tool 디버그 · {name}', cloudCost: '클라우드 비용', assets: '자산', monitoring: '모니터링', grafana: 'Grafana', kubernetes: 'Kubernetes', conversations: 'AI 대화', publicNavigation: 'Public Navigation',
  conversationManagement: '대화 관리', conversationManagementDesc: 'Multi-turn 대화, Model Source와 최근 활동을 한곳에서 확인하고 기존 Context로 다시 전환할 수 있습니다.', newConversation: '새 대화 시작', searchConversationTitle: '대화 제목 검색', search: '검색', conversation: '대화', createdBy: '{name} 생성', currentUser: '현재 사용자', model: 'Model', messageCount: '메시지 수', recentActivity: '최근 활동', status: '상태', resumable: '계속 가능', actions: '작업', continueConversation: '대화 계속', unpin: '고정 해제', pin: '고정', delete: '삭제', conversationUnpinned: '고정을 해제했습니다.', conversationPinned: '고정했습니다.', deleteConversation: '대화 삭제', deleteConversationConfirm: '대화 “{title}”을(를) 삭제하시겠습니까?', conversationDeleted: '대화를 삭제했습니다.'
}

const en = {
  toolRegistry: 'Tool Registry', toolRegistryDesc: 'Manage platform capabilities callable by AI. Read tools may run automatically; every Kubernetes write operation requires explicit human confirmation.', enabledCount: 'Enabled / {total}', capabilityCount: '{count} capabilities', write: 'Write', readOnly: 'Read Only', debug: 'Debug', cancel: 'Cancel', executeReadOnly: 'Execute Read Tool', toolUpdated: 'Tool configuration updated.', toolResult: 'Tool Result', debugTool: 'Debug Tool · {name}', cloudCost: 'Cloud Cost', assets: 'Assets', monitoring: 'Monitoring', grafana: 'Grafana', kubernetes: 'Kubernetes', conversations: 'AI Conversations', publicNavigation: 'Public Navigation',
  conversationManagement: 'Conversation Management', conversationManagementDesc: 'Review multi-turn conversations, model sources, and recent activity in one place, then return to historical context at any time.', newConversation: 'Start New Conversation', searchConversationTitle: 'Search conversation titles', search: 'Search', conversation: 'Conversation', createdBy: 'Created by {name}', currentUser: 'Current User', model: 'Model', messageCount: 'Messages', recentActivity: 'Recent Activity', status: 'Status', resumable: 'Resumable', actions: 'Actions', continueConversation: 'Continue', unpin: 'Unpin', pin: 'Pin', delete: 'Delete', conversationUnpinned: 'Conversation unpinned.', conversationPinned: 'Conversation pinned.', deleteConversation: 'Delete Conversation', deleteConversationConfirm: 'Delete conversation “{title}”?', conversationDeleted: 'Conversation deleted.'
}

export function it(key, params = {}) {
  const dict = currentLocale.value === 'en-US' ? en : ko
  let text = dict[key] || en[key] || key
  Object.entries(params).forEach(([name, value]) => { text = text.replaceAll(`{${name}}`, String(value)) })
  return text
}

const legacyCategoryFragments = [['\u4e91\u8d39\u7528', 'cloudCost'], ['\u8d44\u4ea7', 'assets'], ['\u76d1\u63a7', 'monitoring']]
export function integrationCategoryLabel(value = '') { let result = String(value); for (const [legacy, key] of legacyCategoryFragments) result = result.replaceAll(legacy, it(key)); return result }
export function integrationCategoryKind(value = '') { const name = String(value); if (name.includes('\u4e91\u8d39\u7528') || name.includes('\u8d44\u4ea7') || /cloud|cost|asset/i.test(name)) return 'cost'; if (name.includes('\u76d1\u63a7') || /monitor|metric|alert/i.test(name)) return 'monitor'; if (/grafana/i.test(name)) return 'grafana'; if (/kubernetes|k8s/i.test(name)) return 'kubernetes'; return 'integration' }
