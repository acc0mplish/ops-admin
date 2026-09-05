// 네거티브 컨트롤 ④ — ko/en {token} 플레이스홀더 집합 불일치 (bundle §4.1 G1-2, C2-2, M-3)
// 기대 검출: [C2-2] 'progressNotice' — ko에만 있음 {count} → 비-0 exit
const ko = {
  progressNotice: '총 {count}건을 처리했습니다.',
  commonLabel: '공통 라벨'
}

const en = {
  progressNotice: 'Processing finished.',
  commonLabel: 'Common Label'
}

export function fx(key, params = {}) {
  const dict = currentLocale.value === 'en-US' ? en : ko
  let text = dict[key] || en[key] || key
  Object.entries(params).forEach(([name, value]) => { text = text.replaceAll(`{${name}}`, String(value)) })
  return text
}
