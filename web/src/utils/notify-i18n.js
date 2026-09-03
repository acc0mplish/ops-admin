import { currentLocale } from './i18n-runtime'

const ko = {
  sendLogs: '전송 로그', sendLogsDesc: '각 통지의 전송 상태, 재시도 과정, 플랫폼 응답과 최종 결과를 추적합니다.', refresh: '새로고침',
  keywordPlaceholder: 'Delivery ID / Rule / Channel / Target / Summary', deliveryStatus: '전송 상태', channelType: 'Channel 유형', businessScope: '비즈니스 영역', startTime: '시작 시각', endTime: '종료 시각', to: '부터', search: '검색', reset: '초기화',
  deliveryId: 'Delivery ID', route: 'Notification Route', businessEvent: 'Business Event', summary: '요약', status: '상태', attempts: '시도 횟수', response: '응답', duration: '소요 시간', createdAt: '생성 시각', actions: '작업', detail: '상세', resend: '재전송',
  deliveryDetail: '전송 상세', target: '대상', recentAttempt: '최근 시도', nextRetry: '다음 재시도', httpStatus: 'HTTP 상태', businessCode: 'Business Code', originalDelivery: '원본 전송 기록', requestBody: 'Request Body', platformResponse: '플랫폼 응답',
  resendTitle: '재전송', resendConfirm: '전송 {id}를 기준으로 새 전송 작업을 생성합니다. 기존 로그는 덮어쓰지 않습니다. 계속하시겠습니까?', confirmResend: '재전송 확인', cancel: '취소', resendCreated: '새 전송 {id}를 생성했습니다.',
  pending: '대기', sending: '전송 중', retrying: '재시도 대기', success: '성공', failed: '실패',
  dingtalk: 'DingTalk', wecom: 'WeCom', feishu: 'Feishu', webhook: 'Custom Webhook',
  all: '전체', job: 'Job 오케스트레이션', pipeline: 'CI/CD Pipeline', schedule: 'Scheduled Task', monitor: 'Monitoring Alert',
  notify: '테스트 통지', waiting_approval: '승인 대기', rejected: '거부됨', firing: 'Alert 발생', recovered: 'Alert 복구'
}

const en = {
  sendLogs: 'Send Logs', sendLogsDesc: 'Track delivery status, retries, platform responses, and final results for every notification.', refresh: 'Refresh',
  keywordPlaceholder: 'Delivery ID / Rule / Channel / Target / Summary', deliveryStatus: 'Delivery Status', channelType: 'Channel Type', businessScope: 'Business Scope', startTime: 'Start Time', endTime: 'End Time', to: 'to', search: 'Search', reset: 'Reset',
  deliveryId: 'Delivery ID', route: 'Notification Route', businessEvent: 'Business Event', summary: 'Summary', status: 'Status', attempts: 'Attempts', response: 'Response', duration: 'Duration', createdAt: 'Created At', actions: 'Actions', detail: 'Detail', resend: 'Resend',
  deliveryDetail: 'Delivery Detail', target: 'Target', recentAttempt: 'Latest Attempt', nextRetry: 'Next Retry', httpStatus: 'HTTP Status', businessCode: 'Business Code', originalDelivery: 'Original Delivery', requestBody: 'Request Body', platformResponse: 'Platform Response',
  resendTitle: 'Resend', resendConfirm: 'Create a new delivery task based on delivery {id}. The original log will not be overwritten. Continue?', confirmResend: 'Confirm Resend', cancel: 'Cancel', resendCreated: 'Created new delivery {id}.',
  pending: 'Pending', sending: 'Sending', retrying: 'Waiting to Retry', success: 'Success', failed: 'Failed',
  dingtalk: 'DingTalk', wecom: 'WeCom', feishu: 'Feishu', webhook: 'Custom Webhook',
  all: 'All', job: 'Job Orchestration', pipeline: 'CI/CD Pipeline', schedule: 'Scheduled Tasks', monitor: 'Monitoring Alerts',
  notify: 'Test Notification', waiting_approval: 'Waiting for Approval', rejected: 'Rejected', firing: 'Alert Firing', recovered: 'Alert Recovered'
}

export function nt(key, params = {}) {
  const dict = currentLocale.value === 'en-US' ? en : ko
  let text = dict[key] || en[key] || key
  Object.entries(params).forEach(([name, value]) => { text = text.replaceAll(`{${name}}`, String(value)) })
  return text
}
