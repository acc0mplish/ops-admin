<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { deleteOpsImageRegistry, queryOpsImageRegistryList, saveOpsImageRegistry } from '../../api/ops'

const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const rows = ref([])
const form = reactive({ id: undefined, name: '', address: '', namespace: '', username: '', password: '', status: 1, description: '' })

function resetForm() {
  Object.assign(form, { id: undefined, name: '', address: '', namespace: '', username: '', password: '', status: 1, description: '' })
}

async function loadData() {
  loading.value = true
  try { rows.value = await queryOpsImageRegistryList() } finally { loading.value = false }
}

function openCreate() { resetForm(); dialogVisible.value = true }
function openEdit(row) {
  Object.assign(form, { id: row.id, name: row.name, address: row.address, namespace: row.namespace || '', username: row.username || '', password: '', status: row.status || 1, description: row.description || '' })
  dialogVisible.value = true
}

async function submit() {
  if (!form.name || !form.address) return ElMessage.warning('请填写镜像仓库名称和地址')
  saving.value = true
  try {
    await saveOpsImageRegistry(form)
    ElMessage.success('保存成功')
    dialogVisible.value = false
    await loadData()
  } finally { saving.value = false }
}

async function remove(row) {
  await ElMessageBox.confirm(`确认删除镜像仓库「${row.name}」吗？已配置的流水线阶段需要重新选择仓库。`, '删除确认', { type: 'warning' })
  await deleteOpsImageRegistry(row.id)
  ElMessage.success('已删除')
  await loadData()
}

function imagePrefix(row) { return [row.address, row.namespace, '<应用编码>'].filter(Boolean).join('/') }
onMounted(loadData)
</script>

<template>
  <div class="registry-page">
    <section class="registry-hero">
      <div><span>CONTAINER IMAGE DELIVERY</span><h1>镜像仓库</h1><p>集中维护 Docker 镜像仓库地址；流水线构建与推送阶段从这里选择目标。</p></div>
      <el-button class="create-registry-button" @click="openCreate"><span class="create-registry-icon">＋</span> 新增镜像仓库</el-button>
    </section>
    <section class="registry-card">
      <div class="registry-card-head"><div><strong>仓库列表</strong><span>镜像名称格式：仓库地址 / 命名空间 / 应用编码 : 分支-时间</span></div><el-button link @click="loadData">刷新</el-button></div>
      <el-table :data="rows" v-loading="loading" empty-text="暂无镜像仓库，请先新增一个仓库">
        <el-table-column prop="name" label="名称" min-width="150" />
        <el-table-column label="镜像前缀" min-width="330"><template #default="{ row }"><code>{{ imagePrefix(row) }}</code></template></el-table-column>
        <el-table-column prop="username" label="登录账号" min-width="130"><template #default="{ row }">{{ row.username || '未配置' }}</template></el-table-column>
        <el-table-column label="认证" width="100"><template #default="{ row }"><el-tag :type="row.hasPassword ? 'success' : 'info'">{{ row.hasPassword ? '已配置' : '未配置' }}</el-tag></template></el-table-column>
        <el-table-column label="状态" width="90"><template #default="{ row }"><el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '启用' : '停用' }}</el-tag></template></el-table-column>
        <el-table-column label="操作" width="140"><template #default="{ row }"><el-button link type="primary" @click="openEdit(row)">编辑</el-button><el-button link type="danger" @click="remove(row)">删除</el-button></template></el-table-column>
      </el-table>
    </section>
    <el-dialog v-model="dialogVisible" :title="form.id ? '编辑镜像仓库' : '新增镜像仓库'" width="680px">
      <el-form label-width="110px">
        <el-form-item label="仓库名称" required><el-input v-model="form.name" placeholder="例如：阿里云杭州 ACR" /></el-form-item>
        <el-form-item label="仓库地址" required><el-input v-model="form.address" placeholder="例如：registry.cn-hangzhou.aliyuncs.com" /></el-form-item>
        <el-form-item label="命名空间"><el-input v-model="form.namespace" placeholder="例如：ops-admin（可选）" /></el-form-item>
        <el-form-item label="登录账号"><el-input v-model="form.username" autocomplete="off" placeholder="用于 docker login（可选）" /></el-form-item>
        <el-form-item label="登录密码"><el-input v-model="form.password" type="password" show-password autocomplete="new-password" :placeholder="form.id ? '留空则保持原密码不变' : '用于 docker login（可选）'" /></el-form-item>
        <el-form-item label="状态"><el-radio-group v-model="form.status"><el-radio :value="1">启用</el-radio><el-radio :value="2">停用</el-radio></el-radio-group></el-form-item>
        <el-form-item label="说明"><el-input v-model="form.description" type="textarea" :rows="3" placeholder="说明仓库用途、网络要求或使用范围" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible = false">取消</el-button><el-button type="primary" :loading="saving" @click="submit">保存</el-button></template>
    </el-dialog>
  </div>
</template>

<style scoped>
.registry-page { padding: 20px; min-height: 100%; background: #f4f7fc; }
.registry-hero { display:flex; justify-content:space-between; align-items:center; padding:26px 30px; border-radius:16px; background:linear-gradient(115deg,#1e3a8a,#5b67f1); color:#fff; box-shadow:0 12px 28px rgba(59,82,184,.16); }
.registry-hero span { font-size:12px; font-weight:700; letter-spacing:1.2px; color:#c9d4ff; }.registry-hero h1 { margin:7px 0; font-size:27px; }.registry-hero p { margin:0; color:#d9e3ff; }
.create-registry-button { height:42px; padding:0 19px; border:0 !important; border-radius:9px; color:#3654d9 !important; background:#fff !important; font-size:15px; font-weight:700; box-shadow:0 8px 18px rgba(18,35,112,.23); transition:transform .2s ease, box-shadow .2s ease; }.create-registry-button:hover { color:#243fbb !important; background:#fff !important; transform:translateY(-2px); box-shadow:0 11px 22px rgba(18,35,112,.3); }.create-registry-icon { margin-right:4px; font-size:20px; font-weight:400; vertical-align:-1px; }
.registry-card { margin-top:18px; padding:20px; border-radius:14px; background:#fff; box-shadow:0 8px 22px rgba(31,49,94,.06); }.registry-card-head { display:flex; align-items:center; justify-content:space-between; margin-bottom:16px; }.registry-card-head strong { font-size:18px; color:#172b4d; }.registry-card-head span { display:block; margin-top:5px; color:#7a8da8; font-size:13px; }code { color:#355cc9; background:#f2f6ff; padding:4px 8px; border-radius:4px; }
</style>
