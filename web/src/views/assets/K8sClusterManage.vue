<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete, Edit, Plus, Refresh } from '@element-plus/icons-vue'
import {
  addK8sCluster,
  deleteK8sCluster,
  queryK8sClusterInfo,
  queryK8sClusterList,
  updateK8sCluster
} from '../../api/k8s'

const loading = ref(false)
const submitting = ref(false)
const dialogVisible = ref(false)
const isEdit = ref(false)
const clusterList = ref([])

const form = reactive({
  id: undefined,
  name: '',
  description: '',
  kubeConfig: ''
})

function resetForm() {
  Object.assign(form, {
    id: undefined,
    name: '',
    description: '',
    kubeConfig: ''
  })
}

async function loadClusters() {
  loading.value = true
  try {
    clusterList.value = await queryK8sClusterList()
  } finally {
    loading.value = false
  }
}

function openCreate() {
  isEdit.value = false
  resetForm()
  dialogVisible.value = true
}

async function openEdit(row) {
  const data = await queryK8sClusterInfo(row.id)
  isEdit.value = true
  Object.assign(form, {
    id: data.id,
    name: data.name,
    description: data.description || '',
    kubeConfig: data.kubeConfig || ''
  })
  dialogVisible.value = true
}

async function submit() {
  if (!form.name.trim()) {
    ElMessage.warning('请输入集群名称')
    return
  }
  if (!form.kubeConfig.trim()) {
    ElMessage.warning('请输入 kubeconfig')
    return
  }

  submitting.value = true
  try {
    if (isEdit.value) {
      await updateK8sCluster({ ...form })
      ElMessage.success('集群已更新')
    } else {
      await addK8sCluster({ ...form })
      ElMessage.success('集群已新增')
    }
    dialogVisible.value = false
    await loadClusters()
  } finally {
    submitting.value = false
  }
}

async function handleDelete(row) {
  await ElMessageBox.confirm(`确认删除集群 ${row.name} 吗？`, '提示', { type: 'warning' })
  await deleteK8sCluster(row.id)
  ElMessage.success('集群已删除')
  await loadClusters()
}

function tagType(status) {
  switch (status) {
    case 'running':
      return 'success'
    case 'warning':
      return 'warning'
    case 'offline':
      return 'danger'
    default:
      return 'info'
  }
}

onMounted(async () => {
  await loadClusters()
})
</script>

<template>
  <div class="cluster-manage-page">
    <section class="page-header">
      <div>
        <h2>集群管理</h2>
        <p>录入 Kubernetes 集群，保存时会自动校验 kubeconfig 连通性。</p>
      </div>
      <div class="header-actions">
        <el-button :icon="Refresh" @click="loadClusters">刷新</el-button>
        <el-button type="primary" :icon="Plus" @click="openCreate">新增集群</el-button>
      </div>
    </section>

    <section class="table-panel" v-loading="loading">
      <el-table :data="clusterList" class="cluster-table">
        <el-table-column prop="name" label="集群名称" min-width="180" />
        <el-table-column label="集群状态" width="120">
          <template #default="{ row }">
            <el-tag :type="tagType(row.status)" effect="light">{{ row.statusText }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="apiServer" label="API Server" min-width="260" />
        <el-table-column prop="version" label="K8s 版本" width="140" />
        <el-table-column prop="nodeCount" label="节点数量" width="100" />
        <el-table-column prop="description" label="描述" min-width="180" show-overflow-tooltip />
        <el-table-column label="操作" width="160" fixed="right">
          <template #default="{ row }">
            <div class="row-actions">
              <el-button link type="primary" :icon="Edit" @click="openEdit(row)">编辑</el-button>
              <el-button link type="danger" :icon="Delete" @click="handleDelete(row)">删除</el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>

      <el-empty v-if="!loading && !clusterList.length" description="还没有录入任何 K8s 集群" />
    </section>

    <el-dialog
      v-model="dialogVisible"
      :title="isEdit ? '编辑 K8s 集群' : '新增 K8s 集群'"
      width="760px"
      destroy-on-close
    >
      <el-form label-width="100px">
        <el-form-item label="集群名称" required>
          <el-input v-model="form.name" placeholder="例如：prod-hz-01" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="2"
            placeholder="补充说明这个集群的用途、环境或负责人"
          />
        </el-form-item>
        <el-form-item label="KubeConfig" required>
          <el-input
            v-model="form.kubeConfig"
            type="textarea"
            :rows="14"
            placeholder="请输入完整 kubeconfig，保存时会自动校验连通性并获取 API Server、K8s 版本、集群状态和节点数量"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.cluster-manage-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  padding: 20px;
  border: 1px solid #dbe4f5;
  border-radius: 8px;
  background: #f7f9fd;
}

.page-header h2 {
  margin: 0;
  font-size: 22px;
  color: #0f172a;
}

.page-header p {
  margin: 8px 0 0;
  color: #6b7a93;
  font-size: 13px;
}

.header-actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.table-panel {
  padding: 16px;
  border: 1px solid #e1e8f5;
  border-radius: 8px;
  background: #fff;
}

.cluster-table {
  width: 100%;
}

.row-actions {
  display: flex;
  gap: 12px;
}

@media (max-width: 860px) {
  .page-header {
    flex-direction: column;
  }
}
</style>
