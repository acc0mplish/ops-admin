import { currentLocale } from './i18n-runtime'

const ko = {
  allWorkloads: '전체 Workload',
  batchActions: '일괄 작업',
  batchUpdateImages: 'Image Version 일괄 변경',
  batchScale: '일괄 Scale',
  batchRestart: '일괄 Restart',
  batchDelete: '일괄 삭제'
}

const en = {
  allWorkloads: 'All Workloads',
  batchActions: 'Batch Actions',
  batchUpdateImages: 'Batch Update Image Versions',
  batchScale: 'Batch Scale',
  batchRestart: 'Batch Restart',
  batchDelete: 'Batch Delete'
}

export function kt(key) {
  const dict = currentLocale.value === 'en-US' ? en : ko
  return dict[key] || en[key] || key
}
