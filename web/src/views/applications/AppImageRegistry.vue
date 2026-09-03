<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { deleteOpsImageRegistry, queryOpsImageRegistryList, saveOpsImageRegistry } from '../../api/ops'
import { apt } from '../../utils/application-i18n'

const loading = ref(false), saving = ref(false), dialogVisible = ref(false), rows = ref([])
const form = reactive({ id: undefined, name: '', address: '', namespace: '', username: '', password: '', status: 1, description: '' })
function resetForm() { Object.assign(form, { id: undefined, name: '', address: '', namespace: '', username: '', password: '', status: 1, description: '' }) }
async function loadData() { loading.value = true; try { rows.value = await queryOpsImageRegistryList() } finally { loading.value = false } }
function openCreate() { resetForm(); dialogVisible.value = true }
function openEdit(row) { Object.assign(form, { id: row.id, name: row.name, address: row.address, namespace: row.namespace || '', username: row.username || '', password: '', status: row.status || 1, description: row.description || '' }); dialogVisible.value = true }
async function submit() { if (!form.name || !form.address) return ElMessage.warning(apt('registryRequired')); saving.value = true; try { await saveOpsImageRegistry(form); ElMessage.success(apt('saved')); dialogVisible.value = false; await loadData() } finally { saving.value = false } }
async function remove(row) { await ElMessageBox.confirm(apt('deleteConfirm', { name: row.name }), apt('deleteTitle'), { type: 'warning' }); await deleteOpsImageRegistry(row.id); ElMessage.success(apt('deleted')); await loadData() }
function imagePrefix(row) { return [row.address, row.namespace, apt('appCodePlaceholder')].filter(Boolean).join('/') }
onMounted(loadData)
</script>

<template>
  <div class="registry-page">
    <section class="registry-hero"><div><span>CONTAINER IMAGE DELIVERY</span><h1>{{ apt('imageRegistry') }}</h1><p>{{ apt('imageRegistryDesc') }}</p></div><el-button class="create-registry-button" @click="openCreate"><span class="create-registry-icon">＋</span>{{ apt('addRegistry') }}</el-button></section>
    <section class="registry-card">
      <div class="registry-card-head"><div><strong>{{ apt('registryList') }}</strong><span>{{ apt('imageFormat') }}</span></div><el-button link @click="loadData">{{ apt('refresh') }}</el-button></div>
      <el-table :data="rows" v-loading="loading" :empty-text="apt('noRegistries')">
        <el-table-column prop="name" :label="apt('name')" min-width="150" />
        <el-table-column :label="apt('imagePrefix')" min-width="330"><template #default="{ row }"><code>{{ imagePrefix(row) }}</code></template></el-table-column>
        <el-table-column prop="username" :label="apt('loginAccount')" min-width="130"><template #default="{ row }">{{ row.username || apt('notConfigured') }}</template></el-table-column>
        <el-table-column :label="apt('authentication')" width="100"><template #default="{ row }"><el-tag :type="row.hasPassword ? 'success' : 'info'">{{ row.hasPassword ? apt('configured') : apt('notConfigured') }}</el-tag></template></el-table-column>
        <el-table-column :label="apt('status')" width="90"><template #default="{ row }"><el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? apt('enabled') : apt('disabled') }}</el-tag></template></el-table-column>
        <el-table-column :label="apt('actions')" width="140"><template #default="{ row }"><el-button link type="primary" @click="openEdit(row)">{{ apt('edit') }}</el-button><el-button link type="danger" @click="remove(row)">{{ apt('delete') }}</el-button></template></el-table-column>
      </el-table>
    </section>
    <el-dialog v-model="dialogVisible" :title="form.id ? apt('editRegistry') : apt('addRegistry')" width="680px">
      <el-form label-width="110px">
        <el-form-item :label="apt('registryName')" required><el-input v-model="form.name" :placeholder="apt('registryNameExample')" /></el-form-item>
        <el-form-item :label="apt('registryAddress')" required><el-input v-model="form.address" :placeholder="apt('registryAddressExample')" /></el-form-item>
        <el-form-item :label="apt('namespace')"><el-input v-model="form.namespace" :placeholder="apt('namespaceExample')" /></el-form-item>
        <el-form-item :label="apt('loginAccount')"><el-input v-model="form.username" autocomplete="off" :placeholder="apt('dockerLoginOptional')" /></el-form-item>
        <el-form-item :label="apt('loginPassword')"><el-input v-model="form.password" type="password" show-password autocomplete="new-password" :placeholder="form.id ? apt('keepPassword') : apt('dockerLoginOptional')" /></el-form-item>
        <el-form-item :label="apt('status')"><el-radio-group v-model="form.status"><el-radio :value="1">{{ apt('enabled') }}</el-radio><el-radio :value="2">{{ apt('disabled') }}</el-radio></el-radio-group></el-form-item>
        <el-form-item :label="apt('description')"><el-input v-model="form.description" type="textarea" :rows="3" :placeholder="apt('descriptionPlaceholder')" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible = false">{{ apt('cancel') }}</el-button><el-button type="primary" :loading="saving" @click="submit">{{ apt('save') }}</el-button></template>
    </el-dialog>
  </div>
</template>

<style scoped>
.registry-page{padding:20px;min-height:100%;background:#f4f7fc}.registry-hero{display:flex;justify-content:space-between;align-items:center;padding:26px 30px;border-radius:16px;background:linear-gradient(115deg,#1e3a8a,#5b67f1);color:#fff;box-shadow:0 12px 28px rgba(59,82,184,.16)}.registry-hero span{font-size:12px;font-weight:700;letter-spacing:1.2px;color:#c9d4ff}.registry-hero h1{margin:7px 0;font-size:27px}.registry-hero p{margin:0;color:#d9e3ff}.create-registry-button{height:42px;padding:0 19px;border:0!important;border-radius:9px;color:#3654d9!important;background:#fff!important;font-size:15px;font-weight:700;box-shadow:0 8px 18px rgba(18,35,112,.23);transition:transform .2s ease,box-shadow .2s ease}.create-registry-button:hover{color:#243fbb!important;background:#fff!important;transform:translateY(-2px);box-shadow:0 11px 22px rgba(18,35,112,.3)}.create-registry-icon{margin-right:4px;font-size:20px;font-weight:400;vertical-align:-1px}.registry-card{margin-top:18px;padding:20px;border-radius:14px;background:#fff;box-shadow:0 8px 22px rgba(31,49,94,.06)}.registry-card-head{display:flex;align-items:center;justify-content:space-between;margin-bottom:16px}.registry-card-head strong{font-size:18px;color:#172b4d}.registry-card-head span{display:block;margin-top:5px;color:#7a8da8;font-size:13px}code{color:#355cc9;background:#f2f6ff;padding:4px 8px;border-radius:4px}
</style>
