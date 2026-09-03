import { currentLocale } from './i18n-runtime'

const ko = {
  allWorkloads: '전체 Workload', batchActions: '일괄 작업', batchUpdateImages: 'Image Version 일괄 변경', batchScale: '일괄 Scale', batchRestart: '일괄 Restart', batchDelete: '일괄 삭제',
  workloadType: 'Workload Type', replicas: 'Replica', updated: 'Updated', available: 'Available', resourceSpec: 'Resource Spec', additionalContainerImages: '+{count}개 Container Image', updatePodSettings: 'Pod 설정 변경', delete: '삭제',
  noAvailableContainer: '현재 Pod에 사용 가능한 Container가 없습니다.', selectContainerFirst: 'Container를 먼저 선택하십시오.', connectedTo: '{namespace}/{pod}에 연결했습니다.', container: 'Container', terminalConnectionFailed: 'Pod Terminal 연결에 실패했습니다.', connectionClosed: '연결이 종료되었습니다.', podContainerLoadFailed: 'Pod Container 조회에 실패했습니다.', backToPods: 'Pod 관리로 돌아가기', podTerminal: 'Pod Terminal', tabCompletionHint: 'Tab으로 Command 또는 Path 자동완성', selectContainer: 'Container 선택', connect: '연결', disconnect: '연결 해제', clearScreen: '화면 지우기'
}

const en = {
  allWorkloads: 'All Workloads', batchActions: 'Batch Actions', batchUpdateImages: 'Batch Update Image Versions', batchScale: 'Batch Scale', batchRestart: 'Batch Restart', batchDelete: 'Batch Delete',
  workloadType: 'Workload Type', replicas: 'Replicas', updated: 'Updated', available: 'Available', resourceSpec: 'Resource Spec', additionalContainerImages: '+{count} container images', updatePodSettings: 'Update Pod Settings', delete: 'Delete',
  noAvailableContainer: 'The current Pod has no available containers.', selectContainerFirst: 'Select a container first.', connectedTo: 'Connected to {namespace}/{pod}.', container: 'Container', terminalConnectionFailed: 'Pod terminal connection failed.', connectionClosed: 'Connection closed.', podContainerLoadFailed: 'Failed to load Pod containers.', backToPods: 'Back to Pod Management', podTerminal: 'Pod Terminal', tabCompletionHint: 'Use Tab to complete commands or paths', selectContainer: 'Select Container', connect: 'Connect', disconnect: 'Disconnect', clearScreen: 'Clear Screen'
}

export function kt(key, params = {}) {
  const dict = currentLocale.value === 'en-US' ? en : ko
  let text = dict[key] || en[key] || key
  Object.entries(params).forEach(([name, value]) => { text = text.replaceAll(`{${name}}`, String(value)) })
  return text
}
