// 네거티브 컨트롤 ① — en 키 누락 (bundle §4.1 G1-2, H-4)
// 기대 검출: [C2-1] en 키 누락 'greeting' → 비-0 exit
const ko = {
  greeting: '인사',
  commonLabel: '공통 라벨'
}

const en = {
  commonLabel: 'Common Label'
}

export function fx(key, params = {}) {
  const dict = currentLocale.value === 'en-US' ? en : ko
  let text = dict[key] || en[key] || key
  Object.entries(params).forEach(([name, value]) => { text = text.replaceAll(`{${name}}`, String(value)) })
  return text
}
