<template>
  <div class="env-page">
    <section class="page-head">
      <div>
        <h1>环境模型</h1>
        <p>统一维护 dev / test / prod 等环境，让应用、K8s、数据库和监控围绕同一环境组织。</p>
      </div>
      <el-button type="primary" @click="openForm()">新增环境</el-button>
    </section>

    <section class="toolbar">
      <el-input v-model="query.keyword" clearable placeholder="搜索环境名称 / 标识" @keyup.enter="loadData" />
      <el-select v-model="query.status" clearable placeholder="状态" @change="loadData">
        <el-option label="启用" value="1" />
        <el-option label="禁用" value="2" />
      </el-select>
      <el-button type="primary" @click="loadData">查询</el-button>
      <el-button @click="resetQuery">重置</el-button>
    </section>

    <el-table :data="list" class="data-table">
      <el-table-column prop="name" label="环境名称" min-width="160" />
      <el-table-column prop="code" label="环境标识" width="140" />
      <el-table-column prop="sort" label="排序" width="100" />
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? '启用' : '禁用' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="description" label="说明" min-width="240" show-overflow-tooltip />
      <el-table-column label="操作" width="150" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openForm(row)">编辑</el-button>
          <el-button link type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="form.id ? '编辑环境' : '新增环境'" width="560px">
      <el-form :model="form" label-width="92px">
        <el-form-item label="环境名称" required>
          <el-input v-model="form.name" placeholder="例如：测试环境" />
        </el-form-item>
        <el-form-item label="环境标识" required>
          <el-input v-model="form.code" :disabled="Boolean(form.id)" placeholder="例如：test" />
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="form.sort" :min="0" />
        </el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="form.status">
            <el-radio :label="1">启用</el-radio>
            <el-radio :label="2">禁用</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="说明">
          <el-input v-model="form.description" type="textarea" :rows="3" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { deleteOpsEnvironment, queryOpsEnvironmentList, saveOpsEnvironment } from '../../api/ops'

const query = reactive({ keyword: '', status: '' })
const list = ref([])
const dialogVisible = ref(false)
const form = reactive({ id: 0, name: '', code: '', sort: 0, status: 1, description: '' })

async function loadData() {
  list.value = await queryOpsEnvironmentList(query)
}

function resetQuery() {
  query.keyword = ''
  query.status = ''
  loadData()
}

function openForm(row) {
  Object.assign(form, row || { id: 0, name: '', code: '', sort: 0, status: 1, description: '' })
  dialogVisible.value = true
}

async function submit() {
  await saveOpsEnvironment(form)
  ElMessage.success('保存成功')
  dialogVisible.value = false
  loadData()
}

async function remove(row) {
  await ElMessageBox.confirm(`确认删除环境 ${row.name}？`, '删除确认', { type: 'warning' })
  await deleteOpsEnvironment(row.id)
  ElMessage.success('删除成功')
  loadData()
}

onMounted(loadData)
</script>

<style scoped>
.env-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.page-head,
.toolbar,
.data-table {
  background: #fff;
  border: 1px solid #dfe8f6;
  border-radius: 8px;
}
.page-head {
  display: flex;
  justify-content: space-between;
  padding: 24px;
}
.page-head h1 {
  margin: 0;
  color: #071a3d;
}
.page-head p {
  margin: 8px 0 0;
  color: #6d7f9f;
}
.toolbar {
  display: flex;
  gap: 12px;
  padding: 16px;
}
.toolbar .el-input {
  width: 280px;
}
.toolbar .el-select {
  width: 150px;
}
</style>
