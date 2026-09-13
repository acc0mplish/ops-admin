import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { t, translateRoute } from '../utils/i18n-runtime'

// I5-I (i5-plan §3.8·CW-ML) — MainLayout.vue 1,397행 분할 1/2.
// 태그 뷰(tags-bar) 도메인을 뷰에서 이동했다(원본 좌표는 각 함수 주석).
// 상태 소유는 이 팩토리 1회 호출(main 뷰) — 자식(LayoutHeader)은 page로 수신.
const TAGS_KEY = 'ops-admin-tags-view'
const MAX_TAG_VIEWS = 10

// 원본 MainLayout.vue:154-156
function isPinnedTag(tag) {
  return tag?.path === '/dashboard'
}

// 원본 MainLayout.vue:158-171
function trimTagViews(tags, protectedPath = '') {
  const result = [...tags]
  while (result.length > MAX_TAG_VIEWS) {
    let removeIndex = result.findIndex((item) => !isPinnedTag(item) && item.path !== protectedPath)
    if (removeIndex === -1) {
      removeIndex = result.findIndex((item) => !isPinnedTag(item))
    }
    if (removeIndex === -1) {
      break
    }
    result.splice(removeIndex, 1)
  }
  return result
}

// 원본 MainLayout.vue:173-180
function releaseTagResources(tag) {
  window.dispatchEvent(new CustomEvent('ops-admin:tab-closed', {
    detail: {
      path: tag.path,
      fullPath: tag.fullPath || tag.path
    }
  }))
}

export function useLayoutTags() {
  const route = useRoute()
  const router = useRouter()
  const tagViews = ref(loadTags())
  const tagContextMenu = ref({
    visible: false,
    x: 0,
    y: 0,
    tag: null
  })

  // 원본 MainLayout.vue:127-145
  function loadTags() {
    const defaults = [{ title: t('dashboard'), path: '/dashboard', affix: true }]
    const raw = localStorage.getItem(TAGS_KEY)
    if (!raw) {
      return defaults
    }
    try {
      const parsed = JSON.parse(raw)
      const source = Array.isArray(parsed) && parsed.length ? parsed : defaults
      return trimTagViews(source.map((item) => ({
        ...item,
        fullPath: item.fullPath || item.path,
        title: translateRoute(item.path, item.title),
        affix: isPinnedTag(item)
      })))
    } catch {
      return defaults
    }
  }

  // 원본 MainLayout.vue:147-152
  function saveTags() {
    localStorage.setItem(
      TAGS_KEY,
      JSON.stringify(tagViews.value.map((item) => ({ ...item, affix: isPinnedTag(item) })))
    )
  }

  // 원본 MainLayout.vue:322-348
  function ensureTag(path, title, fullPath = path) {
    if (!path || path === '/login') {
      return
    }
    const translated = translateRoute(path, title || t('untitledPage'))
    const found = tagViews.value.find((item) => item.path === path)
    if (found) {
      found.title = translated
      // 탭은 라우트 경로 기준으로 중복 제거하되 마지막으로 연 query parameter는 유지한다.
      // serviceId처럼 query context에 의존하는 페이지가 탭 전환 후 문맥을 잃지 않게 하기 위함이다.
      found.fullPath = fullPath || path
      saveTags()
      return
    }
    tagViews.value.push({
      title: translated,
      path,
      fullPath: fullPath || path,
      affix: path === '/dashboard'
    })
    const nextTags = trimTagViews(tagViews.value, path)
    tagViews.value
      .filter((item) => !nextTags.some((nextItem) => nextItem.path === item.path))
      .forEach(releaseTagResources)
    tagViews.value = nextTags
    saveTags()
  }

  // 원본 MainLayout.vue:350-355
  function handleTagClick(tag) {
    const target = tag?.fullPath || tag?.path
    if (target && target !== route.fullPath) {
      router.push(target)
    }
  }

  // 원본 MainLayout.vue:357-373
  function closeTag(tag) {
    if (isPinnedTag(tag)) {
      return
    }
    const index = tagViews.value.findIndex((item) => item.path === tag.path)
    if (index === -1) {
      return
    }
    const wasActive = route.path === tag.path
    releaseTagResources(tag)
    tagViews.value.splice(index, 1)
    saveTags()
    if (wasActive) {
      const nextTag = tagViews.value[index - 1] || tagViews.value[index] || tagViews.value[0]
      router.push(nextTag?.fullPath || nextTag?.path || '/dashboard')
    }
  }

  // 원본 MainLayout.vue:375-383
  function openTagContextMenu(event, tag) {
    event.preventDefault()
    tagContextMenu.value = {
      visible: true,
      x: event.clientX,
      y: event.clientY,
      tag
    }
  }

  // 원본 MainLayout.vue:385-387
  function closeTagContextMenu() {
    tagContextMenu.value.visible = false
  }

  // 원본 MainLayout.vue:389-408
  function closeLeftTags() {
    const target = tagContextMenu.value.tag
    if (!target) {
      return
    }
    const targetIndex = tagViews.value.findIndex((item) => item.path === target.path)
    if (targetIndex <= 0) {
      closeTagContextMenu()
      return
    }
    const removedTags = tagViews.value.filter((item, index) => index < targetIndex && !isPinnedTag(item))
    const removedActive = removedTags.some((item) => item.path === route.path)
    removedTags.forEach(releaseTagResources)
    tagViews.value = tagViews.value.filter((item, index) => index >= targetIndex || isPinnedTag(item))
    saveTags()
    closeTagContextMenu()
    if (removedActive) {
      router.push(target.fullPath || target.path)
    }
  }

  // 원본 MainLayout.vue:410-429
  function closeRightTags() {
    const target = tagContextMenu.value.tag
    if (!target) {
      return
    }
    const targetIndex = tagViews.value.findIndex((item) => item.path === target.path)
    if (targetIndex === -1) {
      closeTagContextMenu()
      return
    }
    const removedTags = tagViews.value.filter((item, index) => index > targetIndex && !isPinnedTag(item))
    const removedActive = removedTags.some((item) => item.path === route.path)
    removedTags.forEach(releaseTagResources)
    tagViews.value = tagViews.value.filter((item, index) => index <= targetIndex || isPinnedTag(item))
    saveTags()
    closeTagContextMenu()
    if (removedActive) {
      router.push(target.fullPath || target.path)
    }
  }

  // 원본 MainLayout.vue:431-446
  function closeOtherTags() {
    const target = tagContextMenu.value.tag
    if (!target) {
      return
    }
    const keepPath = target.path
    tagViews.value
      .filter((item) => !isPinnedTag(item) && item.path !== keepPath)
      .forEach(releaseTagResources)
    tagViews.value = tagViews.value.filter((item) => isPinnedTag(item) || item.path === keepPath)
    saveTags()
    closeTagContextMenu()
    if (route.path !== keepPath && !tagViews.value.some((item) => item.path === route.path)) {
      router.push(target.fullPath || keepPath)
    }
  }

  return {
    tagViews,
    tagContextMenu,
    isPinnedTag,
    ensureTag,
    handleTagClick,
    closeTag,
    openTagContextMenu,
    closeTagContextMenu,
    closeLeftTags,
    closeRightTags,
    closeOtherTags
  }
}
