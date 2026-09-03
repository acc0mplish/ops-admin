import { currentLocale } from './i18n-runtime'

const ko = {
  allWorkloads: '전체 Workload', batchActions: '일괄 작업', batchUpdateImages: 'Image Version 일괄 변경', batchScale: '일괄 Scale', batchRestart: '일괄 Restart', batchDelete: '일괄 삭제',
  workloadType: 'Workload Type', replicas: 'Replica', updated: 'Updated', available: 'Available', resourceSpec: 'Resource Spec', additionalContainerImages: '+{count}개 Container Image', updatePodSettings: 'Pod 설정 변경', delete: '삭제'
}

const en = {
  allWorkloads: 'All Workloads', batchActions: 'Batch Actions', batchUpdateImages: 'Batch Update Image Versions', batchScale: 'Batch Scale', batchRestart: 'Batch Restart', batchDelete: 'Batch Delete',
  workloadType: 'Workload Type', replicas: 'Replicas', updated: 'Updated', available: 'Available', resourceSpec: 'Resource Spec', additionalContainerImages: '+{count} container images', updatePodSettings: 'Update Pod Settings', delete: 'Delete'
}

export function kt(key, params = {}) {
  const dict = currentLocale.value === 'en-US' ? en : ko
  let text = dict[key] || en[key] || key
  Object.entries(params).forEach(([name, value]) => { text = text.replaceAll(`{${name}}`, String(value)) })
  return text
}
