// 네거티브 컨트롤 ⑤ — callsite 미참조 키 (bundle §4.1 G1-2, 판단 E — fallback 은폐 차단)
// 'orphanKey'는 ko/en에 쌍으로 존재하지만 어떤 뷰에서도 참조하지 않는 죽은 키.
// 기대 검출: [C2-4b] callsite 미참조 키 1건: 'orphanKey' → 비-0 exit
const ko = {
  commonLabel: '공통 라벨',
  orphanKey: '아무도 부르지 않는 키'
}

const en = {
  commonLabel: 'Common Label',
  orphanKey: 'A key nobody references'
}

export function fx(key, params = {}) {
  const dict = currentLocale.value === 'en-US' ? en : ko
  let text = dict[key] || en[key] || key
  Object.entries(params).forEach(([name, value]) => { text = text.replaceAll(`{${name}}`, String(value)) })
  return text
}
