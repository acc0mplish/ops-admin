<template>
  <div class="env-page">
    <section class="page-head">
      <div>
        <h1>{{ ot('environmentModel') }}</h1>
        <p>{{ ot('envModelDesc') }}</p>
      </div>
      <el-button type="primary" @click="openForm()">{{ ot('newEnvironment') }}</el-button>
    </section>

    <section class="toolbar">
      <el-input v-model="query.keyword" clearable :placeholder="ot('searchEnvironment')" @keyup.enter="loadData" />
      <el-select v-model="query.status" clearable :placeholder="ot('status')" @change="loadData">
        <el-option :label="ot('enabled')" value="1" />
        <el-option :label="ot('disabled')" value="2" />
      </el-select>
      <el-button type="primary" @click="loadData">{{ ot('search') }}</el-button>
      <el-button @click="resetQuery">{{ ot('reset') }}</el-button>
    </section>

    <el-table :data="list" class="data-table">
      <el-table-column prop="name" :label="ot('environmentName')" min-width="160" />
      <el-table-column prop="code" label="Environment Code" width="140" />
      <el-table-column prop="sort" :label="ot('sortLabel')" width="100" />
      <el-table-column :label="ot('status')" width="100">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? ot('enabled') : ot('disabled') }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="description" :label="ot('description')" min-width="240" show-overflow-tooltip />
      <el-table-column :label="ot('actions')" width="150" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openForm(row)">{{ ot('edit') }}</el-button>
          <el-button link type="danger" @click="remove(row)">{{ ot('delete') }}</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="form.id ? ot('editEnvironment') : ot('newEnvironment')" width="560px">
      <el-form :model="form" label-width="92px">
        <el-form-item :label="ot('environmentName')" required>
          <el-input v-model="form.name" :placeholder="ot('envNameExample')" />
        </el-form-item>
        <el-form-item label="Environment Code" required>
          <el-input v-model="form.code" :disabled="Boolean(form.id)" :placeholder="ot('codeExample')" />
        </el-form-item>
        <el-form-item :label="ot('sortLabel')">
          <el-input-number v-model="form.sort" :min="0" />
        </el-form-item>
        <el-form-item :label="ot('status')">
          <el-radio-group v-model="form.status">
            <el-radio :value="1">{{ ot('enabled') }}</el-radio>
            <el-radio :value="2">{{ ot('disabled') }}</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item :label="ot('description')">
          <el-input v-model="form.description" type="textarea" :rows="3" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ ot('cancel') }}</el-button>
        <el-button type="primary" @click="submit">{{ ot('save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { deleteOpsEnvironment, queryOpsEnvironmentList, saveOpsEnvironment } from '../../api/ops'
import { ot } from '../../utils/ops-i18n'

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
  ElMessage.success(ot('savedSuccess'))
  dialogVisible.value = false
  loadData()
}

async function remove(row) {
  await ElMessageBox.confirm(ot('deleteEnvironmentConfirm', { name: row.name }), ot('deleteConfirmTitle'), { type: 'warning' })
  await deleteOpsEnvironment(row.id)
  ElMessage.success(ot('deleteSuccess'))
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
