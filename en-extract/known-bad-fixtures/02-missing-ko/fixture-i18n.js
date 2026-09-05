// 네거티브 컨트롤 ② — ko 키 누락 (bundle §4.1 G1-2, H-4)
// 기대 검출: [C2-1] ko 키 누락 'farewell' → 비-0 exit
const ko = {
  commonLabel: '공통 라벨'
}

const en = {
  commonLabel: 'Common Label',
  farewell: 'Farewell'
}

export function fx(key, params = {}) {
  const dict = currentLocale.value === 'en-US' ? en : ko
  let text = dict[key] || en[key] || key
  Object.entries(params).forEach(([name, value]) => { text = text.replaceAll(`{${name}}`, String(value)) })
  return text
}
