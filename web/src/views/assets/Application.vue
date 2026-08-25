<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useRouter } from 'vue-router'
import { assetServiceInfo, deleteAssetService, queryAssetServiceK8sCatalog, queryAssetServiceList, saveAssetService } from '../../api/asset'
import { queryK8sClusterList } from '../../api/k8s'

const router = useRouter()
const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const rows = ref([])
const clusters = ref([])
const catalog = ref({ namespaces: [], workloads: [] })
const query = reactive({ pageNum: 1, pageSize: 20, keyword: '' })
const form = reactive({ id: undefined, name: '', k8sClusterId: undefined, namespace: '', serviceType: '游戏服务', description: '', status: 1, workloads: [] })

const selectedWorkloadKeys = computed({
  get: () => form.workloads.map((item) => `${item.workloadType}:${item.workloadName}`),
  set: (keys) => {
    form.workloads = (catalog.value.workloads || []).filter((item) => keys.includes(`${String(item.type).toLowerCase()}:${item.name}`)).map((item) => ({ workloadType: String(item.type).toLowerCase(), workloadName: item.name }))
  }
})

function resetForm() {
  Object.assign(form, { id: undefined, name: '', k8sClusterId: undefined, namespace: '', serviceType: '游戏服务', description: '', status: 1, workloads: [] })
  catalog.value = { namespaces: [], workloads: [] }
}

function clusterName(id) { return clusters.value.find((item) => Number(item.id) === Number(id))?.name || `集群 #${id}` }
function workloadKey(item) { return `${String(item.type).toLowerCase()}:${item.name}` }

async function loadData() {
  loading.value = true
  try {
    const data = await queryAssetServiceList(query)
    rows.value = data.list || []
  } finally { loading.value = false }
}

async function loadCatalog(selectAll = false) {
  if (!form.k8sClusterId) { catalog.value = { namespaces: [], workloads: [] }; return }
  const data = await queryAssetServiceK8sCatalog({ clusterId: form.k8sClusterId, namespace: form.namespace || undefined })
  catalog.value = data || { namespaces: [], workloads: [] }
  if (selectAll && form.namespace) {
    form.workloads = (catalog.value.workloads || []).map((item) => ({ workloadType: String(item.type).toLowerCase(), workloadName: item.name }))
  }
}

async function onClusterChange() {
  form.namespace = ''
  form.workloads = []
  await loadCatalog()
}

async function onNamespaceChange() {
  form.workloads = []
  await loadCatalog(true)
}

function openCreate() { resetForm(); dialogVisible.value = true }
async function openEdit(row) {
  resetForm()
  const item = await assetServiceInfo(row.id)
  Object.assign(form, { ...form, ...item, workloads: item.workloads || [] })
  if (form.k8sClusterId) await loadCatalog(false)
  dialogVisible.value = true
}

async function submit() {
  if (!form.name || !form.k8sClusterId || !form.namespace) {
    ElMessage.warning('请填写服务名称，并选择 Kubernetes 集群和命名空间')
    return
  }
  if (!form.workloads.length) { ElMessage.warning('请至少选择一个工作负载'); return }
  saving.value = true
  try {
    await saveAssetService(form)
    ElMessage.success('服务已保存')
    dialogVisible.value = false
    await loadData()
  } finally { saving.value = false }
}

async function remove(row) {
  await ElMessageBox.confirm(`确认删除服务“${row.name}”？已关联的工作负载关系也会删除。`, '删除服务', { type: 'warning' })
  await deleteAssetService(row.id)
  ElMessage.success('服务已删除')
  await loadData()
}

function openTopology(row) { router.push({ path: '/containers/services/topology', query: { serviceId: row.id } }) }

onMounted(async () => {
  const [clusterData] = await Promise.all([queryK8sClusterList(), loadData()])
  clusters.value = clusterData || []
})
</script>

<template>
  <div class="asset-app-page" v-loading="loading">
    <section class="app-hero">
      <div><p class="eyebrow">SERVICE ASSET</p><h1>服务管理</h1><p>从 Kubernetes 命名空间发现工作负载，并将多个工作负载归集为一个可追踪的业务服务。</p></div>
      <el-button type="primary" @click="openCreate">新增服务</el-button>
    </section>

    <section class="filter-card">
      <el-input v-model="query.keyword" clearable placeholder="搜索服务名称、唯一 ID 或命名空间" @keyup.enter="loadData" />
      <el-button type="primary" @click="loadData">查询</el-button><el-button @click="query.keyword = ''; loadData()">重置</el-button>
    </section>

    <section class="table-card">
      <el-table :data="rows" row-key="id" empty-text="暂无服务，请从 Kubernetes 命名空间创建">
        <el-table-column label="服务" min-width="170"><template #default="{ row }"><b>{{ row.name }}</b><small>{{ row.serviceType || '业务服务' }}</small></template></el-table-column>
        <el-table-column prop="serviceUid" label="唯一 ID" min-width="280"><template #default="{ row }"><code>{{ row.serviceUid }}</code></template></el-table-column>
        <el-table-column label="Kubernetes 范围" min-width="220"><template #default="{ row }"><span>{{ clusterName(row.k8sClusterId) }}</span><small>{{ row.namespace || '-' }}</small></template></el-table-column>
        <el-table-column prop="description" label="说明" min-width="180" show-overflow-tooltip />
        <el-table-column label="状态" width="90"><template #default="{ row }"><el-tag :type="Number(row.status) === 1 ? 'success' : 'info'">{{ Number(row.status) === 1 ? '启用' : '禁用' }}</el-tag></template></el-table-column>
        <el-table-column label="操作" width="210" fixed="right"><template #default="{ row }"><el-button link type="primary" @click="openTopology(row)">查看拓扑</el-button><el-button link type="primary" @click="openEdit(row)">编辑</el-button><el-button link type="danger" @click="remove(row)">删除</el-button></template></el-table-column>
      </el-table>
    </section>

    <el-dialog v-model="dialogVisible" :title="form.id ? '编辑服务' : '新增服务'" width="min(960px, 94vw)" destroy-on-close>
      <el-alert type="info" :closable="false" show-icon title="唯一 ID 自动生成：集群地址 - 命名空间 - 服务名称。例如：192.168.171.51-szfc-inner-51服务。" />
      <el-form label-width="110px" class="app-form">
        <el-row :gutter="18"><el-col :span="12"><el-form-item label="服务名称" required><el-input v-model="form.name" placeholder="例如：51服务" /></el-form-item></el-col><el-col :span="12"><el-form-item label="服务类型"><el-input v-model="form.serviceType" placeholder="例如：游戏服务" /></el-form-item></el-col></el-row>
        <el-row :gutter="18"><el-col :span="12"><el-form-item label="K8s 集群" required><el-select v-model="form.k8sClusterId" filterable style="width:100%" placeholder="选择集群" @change="onClusterChange"><el-option v-for="item in clusters" :key="item.id" :label="`${item.name} · ${item.apiServer}`" :value="item.id" /></el-select></el-form-item></el-col><el-col :span="12"><el-form-item label="命名空间" required><el-select v-model="form.namespace" filterable style="width:100%" :disabled="!form.k8sClusterId" placeholder="选择命名空间" @change="onNamespaceChange"><el-option v-for="item in catalog.namespaces || []" :key="item.name" :label="item.name" :value="item.name" /></el-select></el-form-item></el-col></el-row>
        <el-form-item label="工作负载" required><div class="workload-picker"><div class="workload-toolbar"><span>命名空间内 {{ catalog.workloads?.length || 0 }} 个工作负载</span><el-button link type="primary" :disabled="!catalog.workloads?.length" @click="selectedWorkloadKeys = catalog.workloads.map(workloadKey)">全选</el-button></div><el-checkbox-group v-model="selectedWorkloadKeys"><el-checkbox v-for="item in catalog.workloads || []" :key="workloadKey(item)" :label="workloadKey(item)" border><b>{{ item.name }}</b><span>{{ item.type }} · Ready {{ item.ready || '0/0' }}</span></el-checkbox></el-checkbox-group><el-empty v-if="form.namespace && !catalog.workloads?.length" description="该命名空间没有可发现的工作负载" :image-size="56" /></div></el-form-item>
        <el-form-item label="说明"><el-input v-model="form.description" type="textarea" :rows="3" placeholder="可选：记录服务职责、入口协议或负责人" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible = false">取消</el-button><el-button type="primary" :loading="saving" @click="submit">保存服务</el-button></template>
    </el-dialog>
  </div>
</template>

<style scoped>
.asset-app-page{padding:24px;background:#f4f7fc;min-height:100%}.app-hero,.filter-card,.table-card{background:#fff;border:1px solid #e1e9f6;border-radius:18px}.app-hero{display:flex;align-items:center;justify-content:space-between;padding:26px 30px;margin-bottom:16px;background:linear-gradient(120deg,#fff 30%,#eef4ff)}.eyebrow{margin:0 0 6px;color:#3970df;font-weight:800;font-size:12px;letter-spacing:.08em}.app-hero h1{margin:0;color:#0d2851;font-size:26px}.app-hero p:not(.eyebrow){margin:8px 0 0;color:#7183a0}.filter-card{display:flex;gap:10px;padding:16px;margin-bottom:16px}.filter-card .el-input{max-width:360px}.table-card{padding:6px 16px}.el-table small{display:block;margin-top:4px;color:#8392aa}.el-table code{color:#355eac;background:#f2f6ff;padding:3px 6px;border-radius:5px;font-size:12px}.app-form{margin-top:22px}.workload-picker{width:100%;padding:14px;border:1px solid #e2e9f5;border-radius:12px;background:#fbfcff}.workload-toolbar{display:flex;justify-content:space-between;align-items:center;margin-bottom:12px;color:#70819c}.el-checkbox-group{display:flex;flex-wrap:wrap;gap:10px}.el-checkbox{margin-right:0!important}.el-checkbox b{display:block}.el-checkbox span{font-size:12px;color:#8090a8}@media(max-width:720px){.app-hero{align-items:flex-start;gap:18px;flex-direction:column}.filter-card{flex-wrap:wrap}}
</style>
