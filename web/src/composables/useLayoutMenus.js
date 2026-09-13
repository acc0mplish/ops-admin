import { computed } from 'vue'
import { t, translateRoute } from '../utils/i18n-runtime'
import { getAppByRoute } from '../utils/apps'

// I5-I (i5-plan §3.8·CW-ML) — MainLayout.vue 1,397행 분할 1/2.
// 메뉴 정규화·표제 어휘를 뷰에서 이동했다(원본 좌표는 각 함수 주석).
// displayTitle 등 순수 함수는 LayoutSidebar 자식이 직접 임포트한다(CW-ML).
// 원본 MainLayout.vue:243-245
export function displayTitle(item) {
  return item.titleKey ? t(item.titleKey) : translateRoute(item.path, item.title)
}

// 원본 MainLayout.vue:228-241
export function findMenuTitle(menuList, path) {
  for (const item of menuList) {
    if (item.path === path) {
      return item.title
    }
    if (item.children?.length) {
      const match = item.children.find((child) => child.path === path)
      if (match) {
        return match.title
      }
    }
  }
  return ''
}

// 원본 MainLayout.vue:247-253
export function flattenMenus(items = [], parentTitle = '') {
  return items.flatMap((item) => {
    const title = displayTitle(item)
    const current = item.path ? [{ title, parentTitle, path: item.path, icon: item.icon || 'Document' }] : []
    return current.concat(flattenMenus(item.children || [], title))
  })
}

// 원본 MainLayout.vue:272-275
export function isConsoleMenuPath(path) {
  const normalizedPath = String(path || '').trim()
  return !normalizedPath || getAppByRoute(normalizedPath).key === 'console'
}

// 원본 MainLayout.vue:215-219
export function isAppEntryMenu(item) {
  const name = item.menuName || item.title || ''
  const path = item.url || item.path || ''
  return name === '콘솔' || name === 'Console' || path === '/console'
}

// 원본 MainLayout.vue:182-213
export function normalizeBackendMenus(raw) {
  const menuList = (raw || [])
    // 콘솔에는 콘솔 소속 백엔드 메뉴만 노출한다. 백엔드 메뉴에는 자산/컨테이너 등
    // 다른 앱 분기도 포함되므로 앱 루트뿐 아니라 하위 경로까지 앱 기준으로 필터링한다.
    .filter((item) => !isAppEntryMenu(item) && isConsoleMenuPath(item.url || item.path))
    .map((item) => ({
      title: item.menuName,
      path: item.url || '',
      icon: item.icon || 'Menu',
      children: (item.menuSvoList || [])
        .filter((child) => child.menuType !== 3 && !isAppEntryMenu(child) && isConsoleMenuPath(child.url || child.path))
        .map((child) => ({
          title: child.menuName,
          path: child.url || '',
          icon: child.icon || 'Document',
          children: []
        }))
    }))
    // 콘솔 경로도 없고 남은 하위 메뉴도 없는 앱 그룹은 빈 메뉴로 표시하지 않는다.
    .filter((item) => item.path || item.children.length)

  if (!menuList.some((item) => item.path === '/dashboard')) {
    menuList.unshift({
      title: t('dashboard'),
      path: '/dashboard',
      icon: 'House',
      children: []
    })
  }

  return menuList
}

// 원본 MainLayout.vue:221-226
export function normalizeStaticMenus(raw) {
  return (raw || []).map((item) => ({
    ...item,
    children: normalizeStaticMenus(item.children || [])
  }))
}

// 원본 MainLayout.vue:60-65 — rawMenus/currentApp은 뷰 소유, 인자 주입(§5 #11)
export function useLayoutMenus({ rawMenus, currentApp }) {
  const sidebarMenus = computed(() => {
    if (currentApp.value.menuSource === 'backend') {
      return normalizeBackendMenus(rawMenus.value)
    }
    return normalizeStaticMenus(currentApp.value.menus || [])
  })
  return { sidebarMenus }
}
