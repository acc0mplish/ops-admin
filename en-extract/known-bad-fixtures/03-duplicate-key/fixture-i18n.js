// 네거티브 컨트롤 ③ — dict 내 중복 키 정의 (bundle §4.1 G1-2, C2-3)
// JS는 후선언으로 조용히 덮어쓰므로 런타임 오류가 없다 — 파서만 검출 가능.
// 기대 검출: [C2-3] 중복 키 'confirmText' (4 행 재선언) → 비-0 exit
const ko = {
  confirmText: '첫 번째 확인',
  confirmText: '두 번째 확인',
  commonLabel: '공통 라벨'
}

const en = {
  confirmText: 'First Confirm',
  commonLabel: 'Common Label'
}

export function fx(key, params = {}) {
  const dict = currentLocale.value === 'en-US' ? en : ko
  let text = dict[key] || en[key] || key
  Object.entries(params).forEach(([name, value]) => { text = text.replaceAll(`{${name}}`, String(value)) })
  return text
}
