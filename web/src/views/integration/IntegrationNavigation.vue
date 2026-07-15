<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  deleteNavigation,
  deleteNavigationGroup,
  queryNavigationGroups,
  queryNavigations,
  regenerateNavigationGroupToken,
  saveNavigation,
  saveNavigationGroup
} from '../../api/integration'

const loading = ref(false)
const groups = ref([])
const navigations = ref([])
const selectedGroupId = ref(0)
const keyword = ref('')
const groupDialogVisible = ref(false)
const navigationDialogVisible = ref(false)
const saving = ref(false)

const groupForm = reactive({ id: undefined, name: '', description: '', isPublic: false, status: 1, sort: 0 })
const navigationForm = reactive({
  id: undefined,
  groupId: 0,
  name: '',
  description: '',
  url: '',
  iconUrl: '',
  openMode: 'new',
  status: 1,
  sort: 0
})

const selectedGroup = computed(() => groups.value.find((item) => item.id === selectedGroupId.value))
const totalNavigationCount = computed(() => groups.value.reduce((total, item) => total + Number(item.itemCount || 0), 0))
const publicGroupCount = computed(() => groups.value.filter((item) => item.isPublic).length)
const publicURL = computed(() => {
  if (!selectedGroup.value?.isPublic || !selectedGroup.value?.publicToken) return ''
  return `${window.location.origin}/public/navigation/${selectedGroup.value.publicToken}`
})

function resetGroupForm() {
  Object.assign(groupForm, { id: undefined, name: '', description: '', isPublic: false, status: 1, sort: 0 })
}

function resetNavigationForm() {
  Object.assign(navigationForm, {
    id: undefined,
    groupId: selectedGroupId.value,
    name: '',
    description: '',
    url: '',
    iconUrl: '',
    openMode: 'new',
    status: 1,
    sort: 0
  })
}

async function loadGroups(preferredId = selectedGroupId.value) {
  const data = await queryNavigationGroups()
  groups.value = data || []
  if (preferredId && groups.value.some((item) => item.id === preferredId)) {
    selectedGroupId.value = preferredId
  } else {
    selectedGroupId.value = groups.value[0]?.id || 0
  }
}

async function loadNavigations() {
  if (!selectedGroupId.value) {
    navigations.value = []
    return
  }
  loading.value = true
  try {
    const data = await queryNavigations({ groupId: selectedGroupId.value, keyword: keyword.value })
    navigations.value = data || []
  } finally {
    loading.value = false
  }
}

async function selectGroup(id) {
  selectedGroupId.value = id
  keyword.value = ''
  await loadNavigations()
}

function openCreateGroup() {
  resetGroupForm()
  groupDialogVisible.value = true
}

function openEditGroup() {
  if (!selectedGroup.value) return
  Object.assign(groupForm, selectedGroup.value)
  groupDialogVisible.value = true
}

async function submitGroup() {
  if (!groupForm.name.trim()) {
    ElMessage.warning('请输入导航组名称')
    return
  }
  saving.value = true
  try {
    const data = await saveNavigationGroup(groupForm)
    groupDialogVisible.value = false
    await loadGroups(data.id)
    await loadNavigations()
    ElMessage.success('导航组已保存')
  } finally {
    saving.value = false
  }
}

async function removeGroup() {
  if (!selectedGroup.value) return
  await ElMessageBox.confirm(`删除“${selectedGroup.value.name}”后，组内导航也会一并删除，确认继续吗？`, '删除导航组', {
    type: 'warning',
    confirmButtonText: '删除',
    cancelButtonText: '取消'
  })
  await deleteNavigationGroup(selectedGroup.value.id)
  await loadGroups(0)
  await loadNavigations()
  ElMessage.success('导航组已删除')
}

function openCreateNavigation() {
  if (!selectedGroupId.value) {
    ElMessage.warning('请先创建导航组')
    return
  }
  resetNavigationForm()
  navigationDialogVisible.value = true
}

function openEditNavigation(item) {
  Object.assign(navigationForm, item)
  navigationDialogVisible.value = true
}

async function submitNavigation() {
  if (!navigationForm.name.trim() || !navigationForm.url.trim()) {
    ElMessage.warning('请输入导航名称和访问地址')
    return
  }
  saving.value = true
  try {
    await saveNavigation(navigationForm)
    navigationDialogVisible.value = false
    await loadGroups(selectedGroupId.value)
    await loadNavigations()
    ElMessage.success('导航已保存')
  } finally {
    saving.value = false
  }
}

async function removeNavigation(item) {
  await ElMessageBox.confirm(`确认删除导航“${item.name}”吗？`, '删除导航', { type: 'warning' })
  await deleteNavigation(item.id)
  await loadGroups(selectedGroupId.value)
  await loadNavigations()
  ElMessage.success('导航已删除')
}

function openNavigation(item) {
  if (item.openMode === 'current') {
    window.location.href = item.url
    return
  }
  const target = window.open(item.url, '_blank', 'noopener,noreferrer')
  if (target) target.opener = null
}

async function copyPublicURL() {
  if (!publicURL.value) return
  await navigator.clipboard.writeText(publicURL.value)
  ElMessage.success('公开链接已复制')
}

async function regenerateToken() {
  await ElMessageBox.confirm('重新生成后，原公开链接将立即失效，确认继续吗？', '重新生成公开链接', { type: 'warning' })
  await regenerateNavigationGroupToken(selectedGroupId.value)
  await loadGroups(selectedGroupId.value)
  ElMessage.success('公开链接已重新生成')
}

function iconText(name) {
  return String(name || 'N').trim().slice(0, 1).toUpperCase()
}

function displayHost(value) {
  try {
    return new URL(value).host
  } catch {
    return value || '-'
  }
}

function toneClass(item) {
  return `tone-${Math.abs(Number(item?.id) || 0) % 5}`
}

onMounted(async () => {
  await loadGroups()
  await loadNavigations()
})
</script>

<template>
  <div class="integration-page">
    <section class="integration-header">
      <div class="header-copy">
        <span class="eyebrow">INTEGRATION HUB</span>
        <h1>集成中心</h1>
        <p>集中管理内部系统、运维工具和第三方平台入口，并按使用场景分组共享。</p>
      </div>
      <div class="header-summary" aria-label="集成中心统计">
        <div><strong>{{ groups.length }}</strong><span>导航分组</span></div>
        <div><strong>{{ totalNavigationCount }}</strong><span>系统入口</span></div>
        <div><strong>{{ publicGroupCount }}</strong><span>公开分组</span></div>
      </div>
      <div class="header-actions">
        <el-button @click="openCreateGroup">新增分组</el-button>
        <el-button type="primary" :disabled="!selectedGroupId" @click="openCreateNavigation">新增导航</el-button>
      </div>
    </section>

    <section class="integration-workspace">
      <aside class="group-panel">
        <div class="panel-title">
          <div><strong>导航分组</strong><small>按使用场景组织入口</small></div>
          <el-button link type="primary" @click="openCreateGroup">新增</el-button>
        </div>
        <div v-if="groups.length" class="group-list">
          <button
            v-for="group in groups"
            :key="group.id"
            type="button"
            class="group-item"
            :class="{ active: selectedGroupId === group.id }"
            @click="selectGroup(group.id)"
          >
            <span class="group-mark" :class="toneClass(group)">{{ iconText(group.name) }}</span>
            <span class="group-copy">
              <strong>{{ group.name }}</strong>
              <small>{{ group.itemCount }} 个导航</small>
            </span>
            <el-tag v-if="group.isPublic" size="small" type="success" effect="light">公开</el-tag>
          </button>
        </div>
        <el-empty v-else description="暂无导航组" :image-size="72" />
      </aside>

      <main class="navigation-panel">
        <template v-if="selectedGroup">
          <div class="group-heading">
            <div>
              <span class="section-label">当前分组</span>
              <div class="heading-line">
                <h2>{{ selectedGroup.name }}</h2>
                <el-tag :type="selectedGroup.status === 1 ? 'success' : 'info'" effect="light">
                  {{ selectedGroup.status === 1 ? '启用' : '禁用' }}
                </el-tag>
              </div>
              <p>{{ selectedGroup.description || '暂无分组说明' }}</p>
            </div>
            <div class="group-actions">
              <el-button @click="openEditGroup">编辑分组</el-button>
              <el-button type="danger" plain @click="removeGroup">删除分组</el-button>
            </div>
          </div>

          <div v-if="publicURL" class="public-link-bar">
            <div class="public-status"><span></span><strong>公开访问已开启</strong></div>
            <code>{{ publicURL }}</code>
            <el-button size="small" @click="copyPublicURL">复制链接</el-button>
            <el-button size="small" @click="regenerateToken">重新生成</el-button>
          </div>

          <div class="navigation-toolbar">
            <el-input v-model="keyword" clearable placeholder="搜索导航名称、说明或地址" @keyup.enter="loadNavigations" />
            <el-button @click="loadNavigations">搜索</el-button>
            <span class="navigation-count"><strong>{{ navigations.length }}</strong> 个导航</span>
          </div>

          <div v-loading="loading" class="navigation-grid">
            <article v-for="item in navigations" :key="item.id" class="navigation-card" :class="toneClass(item)" @click="openNavigation(item)">
              <div class="card-main">
              <div class="navigation-icon" :class="toneClass(item)">
                <img v-if="item.iconUrl" :src="item.iconUrl" alt="" />
                <span v-else>{{ iconText(item.name) }}</span>
              </div>
              <div class="navigation-info">
                <div class="navigation-name">
                  <strong>{{ item.name }}</strong>
                  <el-tag v-if="item.status !== 1" size="small" type="info">禁用</el-tag>
                </div>
                <p>{{ item.description || '暂无入口说明' }}</p>
              </div>
              <span class="open-arrow">↗</span>
              </div>
              <div class="card-footer">
                <span class="host-name">{{ displayHost(item.url) }}</span>
                <div class="card-actions" @click.stop>
                <el-button link type="primary" @click="openEditNavigation(item)">编辑</el-button>
                <el-button link type="danger" @click="removeNavigation(item)">删除</el-button>
                </div>
              </div>
            </article>
            <button type="button" class="navigation-card add-card" @click="openCreateNavigation">
              <span class="add-symbol">+</span><strong>新增导航</strong><small>添加新的系统或工具入口</small>
            </button>
          </div>
        </template>
        <el-empty v-else description="创建一个导航组后开始添加导航" />
      </main>
    </section>

    <el-dialog v-model="groupDialogVisible" :title="groupForm.id ? '编辑导航组' : '新增导航组'" width="620px" destroy-on-close>
      <el-form label-position="top">
        <div class="form-grid two-columns">
          <el-form-item label="导航组名称" required><el-input v-model="groupForm.name" maxlength="128" /></el-form-item>
          <el-form-item label="排序"><el-input-number v-model="groupForm.sort" :min="0" :max="9999" controls-position="right" /></el-form-item>
        </div>
        <el-form-item label="分组说明"><el-input v-model="groupForm.description" type="textarea" :rows="3" maxlength="500" show-word-limit /></el-form-item>
        <div class="form-grid two-columns">
          <el-form-item label="状态">
            <el-radio-group v-model="groupForm.status"><el-radio :value="1">启用</el-radio><el-radio :value="2">禁用</el-radio></el-radio-group>
          </el-form-item>
          <el-form-item label="公开访问">
            <el-switch v-model="groupForm.isPublic" active-text="允许免登录访问" />
          </el-form-item>
        </div>
        <el-alert v-if="groupForm.isPublic" type="warning" :closable="false" show-icon title="公开后，任何获得链接的人都可以查看并打开组内启用的导航。" />
      </el-form>
      <template #footer><el-button @click="groupDialogVisible = false">取消</el-button><el-button type="primary" :loading="saving" @click="submitGroup">保存</el-button></template>
    </el-dialog>

    <el-dialog v-model="navigationDialogVisible" :title="navigationForm.id ? '编辑导航' : '新增导航'" width="680px" destroy-on-close>
      <el-form label-position="top">
        <div class="form-grid two-columns">
          <el-form-item label="导航名称" required><el-input v-model="navigationForm.name" maxlength="128" /></el-form-item>
          <el-form-item label="所属分组" required>
            <el-select v-model="navigationForm.groupId" style="width: 100%"><el-option v-for="group in groups" :key="group.id" :label="group.name" :value="group.id" /></el-select>
          </el-form-item>
        </div>
        <el-form-item label="访问地址" required><el-input v-model="navigationForm.url" placeholder="https://example.com" /></el-form-item>
        <el-form-item label="图标地址"><el-input v-model="navigationForm.iconUrl" placeholder="可选，填写 HTTP/HTTPS 图片地址；留空显示名称首字母" /></el-form-item>
        <el-form-item label="导航说明"><el-input v-model="navigationForm.description" type="textarea" :rows="2" maxlength="500" show-word-limit /></el-form-item>
        <div class="form-grid three-columns">
          <el-form-item label="打开方式"><el-radio-group v-model="navigationForm.openMode"><el-radio value="new">新窗口</el-radio><el-radio value="current">当前窗口</el-radio></el-radio-group></el-form-item>
          <el-form-item label="状态"><el-switch v-model="navigationForm.status" :active-value="1" :inactive-value="2" active-text="启用" /></el-form-item>
          <el-form-item label="排序"><el-input-number v-model="navigationForm.sort" :min="0" :max="9999" controls-position="right" /></el-form-item>
        </div>
      </el-form>
      <template #footer><el-button @click="navigationDialogVisible = false">取消</el-button><el-button type="primary" :loading="saving" @click="submitNavigation">保存</el-button></template>
    </el-dialog>
  </div>
</template>

<style scoped>
.integration-page { display: flex; flex-direction: column; gap: 14px; color: #10213e; }
.integration-header { display: grid; grid-template-columns: minmax(360px, 1fr) auto auto; align-items: center; gap: 28px; padding: 22px 26px; background: #fff; border: 1px solid #e0e8f3; border-radius: 8px; box-shadow: 0 7px 22px rgba(45, 72, 110, .05); }
.header-copy { min-width: 0; }
.eyebrow, .section-label { color: #356fd6; font-size: 11px; font-weight: 800; letter-spacing: 0; }
.integration-header h1 { margin: 4px 0; font-size: 27px; line-height: 1.2; }
.integration-header p, .group-heading p { margin: 0; color: #74839d; }
.header-summary { display: flex; align-items: stretch; height: 48px; }
.header-summary > div { display: flex; min-width: 88px; padding: 0 18px; border-left: 1px solid #e6ebf3; flex-direction: column; justify-content: center; }
.header-summary strong { color: #173e79; font-size: 20px; line-height: 1.1; }
.header-summary span { margin-top: 4px; color: #8996aa; font-size: 12px; }
.header-actions { display: flex; flex-shrink: 0; }
.integration-workspace { display: grid; grid-template-columns: 244px minmax(0, 1fr); min-height: 630px; overflow: hidden; background: #fff; border: 1px solid #e0e8f3; border-radius: 8px; box-shadow: 0 8px 25px rgba(45, 72, 110, .04); }
.group-panel { padding: 17px 10px; border-right: 1px solid #e5ebf4; background: #f7f9fd; }
.panel-title { display: flex; align-items: center; justify-content: space-between; padding: 0 9px 13px; }
.panel-title div { display: flex; flex-direction: column; gap: 3px; }
.panel-title small { color: #8a97ac; font-size: 12px; }
.group-list { display: flex; flex-direction: column; gap: 5px; }
.group-item { display: flex; align-items: center; gap: 10px; width: 100%; min-height: 54px; padding: 9px; border: 1px solid transparent; border-radius: 7px; color: #34445f; background: transparent; text-align: left; cursor: pointer; transition: background-color .16s, border-color .16s; }
.group-item:hover { border-color: #dce7f7; background: #fff; }
.group-item.active { border-color: #b8cff4; background: #fff; color: #1e5dbb; box-shadow: 0 4px 12px rgba(40, 89, 154, .07); }
.group-mark { display: grid; place-items: center; width: 34px; height: 34px; flex: 0 0 34px; border-radius: 7px; background: #e5efff; color: #2563c3; font-weight: 800; }
.group-copy { display: flex; min-width: 0; flex: 1; flex-direction: column; gap: 3px; }
.group-copy strong, .group-copy small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.group-copy small { color: #8996aa; }
.navigation-panel { min-width: 0; padding: 22px 24px 28px; }
.group-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 20px; padding-bottom: 16px; border-bottom: 1px solid #edf1f6; }
.heading-line { display: flex; align-items: center; gap: 10px; margin-top: 4px; }
.heading-line h2 { margin: 0 0 4px; font-size: 22px; }
.group-actions { display: flex; flex-shrink: 0; }
.public-link-bar { display: grid; grid-template-columns: auto minmax(120px, 1fr) auto auto; align-items: center; gap: 9px; margin-top: 14px; padding: 9px 11px; border: 1px solid #bce5cd; border-radius: 7px; background: #f3fbf6; }
.public-status { display: flex; align-items: center; gap: 7px; color: #187847; white-space: nowrap; }
.public-status span { width: 7px; height: 7px; border-radius: 50%; background: #22b66c; }
.public-link-bar code { overflow: hidden; color: #46617c; text-overflow: ellipsis; white-space: nowrap; }
.navigation-toolbar { display: flex; align-items: center; gap: 9px; margin: 14px 0; }
.navigation-toolbar .el-input { width: min(420px, 55%); }
.navigation-count { margin-left: auto; color: #8996aa; }
.navigation-count strong { color: #2a4e7f; }
.navigation-grid { display: grid; grid-template-columns: repeat(4, minmax(210px, 1fr)); gap: 11px; align-content: start; min-height: 300px; }
.navigation-card { position: relative; display: flex; min-width: 0; min-height: 138px; padding: 0; overflow: hidden; border: 1px solid #dfe7f1; border-top: 3px solid #5b8def; border-radius: 7px; color: inherit; background: #fff; flex-direction: column; cursor: pointer; transition: border-color .18s, box-shadow .18s, transform .18s; }
.navigation-card:hover { transform: translateY(-2px); border-color: #a7c1ea; box-shadow: 0 9px 22px rgba(34, 82, 145, .1); }
.card-main { display: flex; min-width: 0; flex: 1; align-items: flex-start; gap: 12px; padding: 15px 14px 11px; }
.navigation-icon { display: grid; place-items: center; width: 42px; height: 42px; flex: 0 0 42px; overflow: hidden; border-radius: 8px; background: #eaf2ff; color: #2f6ed3; font-size: 18px; font-weight: 800; }
.navigation-icon img { width: 30px; height: 30px; object-fit: contain; }
.navigation-info { min-width: 0; flex: 1; }
.navigation-name { display: flex; align-items: center; gap: 6px; min-height: 24px; }
.navigation-name strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.navigation-info p { display: -webkit-box; min-height: 35px; margin: 6px 0 0; overflow: hidden; color: #6b7b92; font-size: 13px; line-height: 1.4; -webkit-box-orient: vertical; -webkit-line-clamp: 2; }
.open-arrow { color: #9aa8ba; font-size: 16px; }
.card-footer { display: flex; height: 36px; padding: 0 11px 0 14px; border-top: 1px solid #eef2f7; background: #fafbfd; align-items: center; justify-content: space-between; gap: 8px; }
.host-name { min-width: 0; overflow: hidden; color: #8492a6; font-family: ui-monospace, SFMono-Regular, Consolas, monospace; font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
.card-actions { display: flex; flex: 0 0 auto; }
.card-actions :deep(.el-button) { margin-left: 7px; font-size: 12px; }
.tone-1 { border-top-color: #21a675; }
.tone-2 { border-top-color: #de8a24; }
.tone-3 { border-top-color: #18a3b7; }
.tone-4 { border-top-color: #7359c9; }
.navigation-icon.tone-1, .group-mark.tone-1 { color: #14805b; background: #e5f6ef; }
.navigation-icon.tone-2, .group-mark.tone-2 { color: #b76c12; background: #fff2df; }
.navigation-icon.tone-3, .group-mark.tone-3 { color: #127d8c; background: #e5f7fa; }
.navigation-icon.tone-4, .group-mark.tone-4 { color: #6046b3; background: #eeeafe; }
.add-card { justify-content: center; gap: 5px; min-height: 138px; border-top-width: 1px; border-style: dashed; color: #6f7f97; align-items: center; }
.add-symbol { font-size: 26px; line-height: 1; color: #477fd8; }
.add-card small { color: #9aa5b5; }
.form-grid { display: grid; gap: 16px; }
.two-columns { grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); }
.three-columns { grid-template-columns: 1.4fr 1fr 1fr; }
@media (max-width: 1450px) { .navigation-grid { grid-template-columns: repeat(3, minmax(220px, 1fr)); } .integration-header { grid-template-columns: minmax(320px, 1fr) auto; } .header-summary { display: none; } }
@media (max-width: 1100px) { .navigation-grid { grid-template-columns: repeat(2, minmax(220px, 1fr)); } }
@media (max-width: 800px) { .integration-header { grid-template-columns: 1fr; } .header-actions, .group-heading { align-items: flex-start; } .group-heading { flex-direction: column; } .integration-workspace { grid-template-columns: 1fr; } .group-panel { border-right: 0; border-bottom: 1px solid #e7edf6; } .group-list { flex-direction: row; overflow-x: auto; } .group-item { min-width: 210px; } .navigation-grid { grid-template-columns: 1fr; } .public-link-bar { grid-template-columns: 1fr auto; } .public-link-bar code { grid-column: 1 / -1; } .form-grid { grid-template-columns: 1fr; } }
</style>
