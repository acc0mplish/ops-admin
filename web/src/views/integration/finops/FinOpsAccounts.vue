<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import FinOpsHeader from './FinOpsHeader.vue'
import {
  deleteFinOpsAccount,
  queryFinOpsAccounts,
  saveFinOpsAccount,
  testFinOpsAccount
} from '../../../api/integration'
import { fat } from '../../../utils/finops-account-i18n'
import './finops.css'

const providers = computed(() => [
  ['alicloud', fat('aliyunOfficial')],
  ['tencent', fat('tencentOfficial')],
  ['aws', fat('awsCustom')],
  ['azure', fat('azureCustom')],
  ['gcp', fat('gcpCustom')],
  ['custom', fat('customAdapter')]
])
const frequencies = computed(() => [
  ['manual', fat('manual')],
  ['hourly', fat('hourly')],
  ['daily', fat('daily')],
  ['weekly', fat('weekly')],
  ['monthly', fat('monthly')]
])

const rows = ref([])
const loading = ref(false)
const visible = ref(false)
const saving = ref(false)
const keyword = ref('')
const provider = ref('')

const initial = () => ({
  id: 0,
  name: '',
  provider: 'alicloud',
  accountIdentifier: '',
  accessKey: '',
  secretKey: '',
  region: '',
  currency: 'CNY',
  billingEndpoint: '',
  billingToken: '',
  syncEnabled: false,
  syncFrequency: 'daily',
  status: 1,
  description: ''
})
const form = reactive(initial())
const builtinBillingProvider = computed(() => ['alicloud', 'tencent'].includes(form.provider))
const accessKeyLabel = computed(() => form.provider === 'tencent' ? 'SecretId' : 'Access Key')
const billingProviderHint = computed(() => {
  if (form.provider === 'alicloud') return fat('aliyunHint')
  if (form.provider === 'tencent') return fat('tencentHint')
  return ''
})

async function load() {
  loading.value = true
  try {
    rows.value = await queryFinOpsAccounts({ keyword: keyword.value, provider: provider.value }) || []
  } finally {
    loading.value = false
  }
}

function create() {
  Object.assign(form, initial())
  visible.value = true
}

function edit(row) {
  Object.assign(form, initial(), row, { accessKey: '', secretKey: '', billingToken: '' })
  visible.value = true
}

async function save() {
  if (!form.name || !form.provider) {
    ElMessage.warning(fat('accountProviderRequired'))
    return
  }
  saving.value = true
  try {
    await saveFinOpsAccount(form)
    visible.value = false
    await load()
    ElMessage.success(fat('accountSaved'))
  } finally {
    saving.value = false
  }
}

async function test(row = form) {
  const result = await testFinOpsAccount({ ...row })
  ElMessage.success(fat('billingConnectionPassed', { source: result?.source ? `, ${result.source}` : '' }))
}

function handleProviderChange() {
  if (builtinBillingProvider.value) {
    form.billingEndpoint = ''
    form.billingToken = ''
  }
}

async function remove(row) {
  await ElMessageBox.confirm(fat('deleteAccountConfirm', { name: row.name }), fat('deleteAccountTitle'), { type: 'warning' })
  await deleteFinOpsAccount(row.id)
  await load()
  ElMessage.success(fat('deleted'))
}

const providerName = value => providers.value.find(item => item[0] === value)?.[1] || value
const frequencyName = value => frequencies.value.find(item => item[0] === value)?.[1] || value

onMounted(load)
</script>

<template>
  <div class="finops-page">
    <FinOpsHeader />
    <section class="finops-panel">
      <div class="finops-head">
        <div>
          <h2>{{ fat('cloudAccounts') }}</h2>
          <p>{{ fat('cloudAccountsDesc') }}</p>
        </div>
        <el-button type="primary" @click="create">{{ fat('addAccount') }}</el-button>
      </div>
      <div class="finops-filter">
        <el-input v-model="keyword" clearable :placeholder="fat('searchNameIdentifier')" style="width: 260px" @keyup.enter="load" />
        <el-select v-model="provider" clearable :placeholder="fat('allProviders')" style="width: 160px">
          <el-option v-for="item in providers" :key="item[0]" :label="item[1]" :value="item[0]" />
        </el-select>
        <el-button @click="load">{{ fat('query') }}</el-button>
      </div>
      <el-table v-loading="loading" :data="rows" style="margin-top: 16px">
        <el-table-column prop="name" :label="fat('accountName')" min-width="150" />
        <el-table-column :label="fat('cloudProvider')" width="110">
          <template #default="{ row }"><span class="finops-provider">{{ providerName(row.provider) }}</span></template>
        </el-table-column>
        <el-table-column :label="fat('billingIntegration')" width="140">
          <template #default="{ row }">
            <el-tag size="small" :type="row.billingCapability?.mode === 'builtin' ? 'success' : 'info'">{{ row.billingCapability?.label || (row.billingCapability?.mode === 'builtin' ? fat('builtinOfficialApi') : fat('billingAdapter')) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="accountIdentifier" :label="fat('accountIdentifier')" min-width="140" />
        <el-table-column prop="region" :label="fat('defaultRegion')" width="130" />
        <el-table-column prop="currency" :label="fat('currency')" width="75" />
        <el-table-column :label="fat('syncPolicy')" width="140">
          <template #default="{ row }">{{ row.syncEnabled ? frequencyName(row.syncFrequency) : fat('notEnabled') }}</template>
        </el-table-column>
        <el-table-column :label="fat('lastSync')" min-width="170">
          <template #default="{ row }">{{ row.lastSyncAt || '-' }}</template>
        </el-table-column>
        <el-table-column :label="fat('status')" width="90">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'info'">{{ row.status === 1 ? fat('enabled') : fat('disabled') }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="fat('actions')" width="190" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="test(row)">{{ fat('validate') }}</el-button>
            <el-button link type="primary" @click="edit(row)">{{ fat('edit') }}</el-button>
            <el-button link type="danger" @click="remove(row)">{{ fat('delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </section>

    <el-dialog v-model="visible" :title="form.id ? fat('editAccount') : fat('newAccount')" width="920px">
      <el-alert :title="fat('credentialsEncryptedHint')" type="info" :closable="false" show-icon />
      <el-form label-position="top" style="margin-top: 18px">
        <el-row :gutter="18">
          <el-col :span="8"><el-form-item :label="fat('accountName')" required><el-input v-model="form.name" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item :label="fat('cloudProvider')" required><el-select v-model="form.provider" style="width: 100%" @change="handleProviderChange"><el-option v-for="item in providers" :key="item[0]" :label="item[1]" :value="item[0]" /></el-select></el-form-item></el-col>
          <el-col :span="8"><el-form-item :label="fat('accountIdentifier')"><el-input v-model="form.accountIdentifier" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item :label="accessKeyLabel"><el-input v-model="form.accessKey" :placeholder="form.id ? fat('leaveBlankToKeep') : ''" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="Secret Key"><el-input v-model="form.secretKey" show-password :placeholder="form.id ? fat('leaveBlankToKeep') : ''" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item :label="fat('defaultRegion')"><el-input v-model="form.region" :placeholder="fat('regionExample')" /></el-form-item></el-col>
          <el-col v-if="builtinBillingProvider" :span="24"><el-alert :title="billingProviderHint" type="success" :closable="false" show-icon /></el-col>
          <template v-else>
            <el-col :span="16"><el-form-item :label="fat('billingHttpEndpoint')"><el-input v-model="form.billingEndpoint" :placeholder="fat('billingEndpointPlaceholder')" /></el-form-item></el-col>
            <el-col :span="8"><el-form-item label="Bearer Token"><el-input v-model="form.billingToken" show-password :placeholder="form.id ? fat('leaveBlankToKeep') : ''" /></el-form-item></el-col>
          </template>
          <el-col :span="8"><el-form-item :label="fat('currency')"><el-select v-model="form.currency" style="width: 100%"><el-option label="CNY" value="CNY" /><el-option label="USD" value="USD" /><el-option label="EUR" value="EUR" /></el-select></el-form-item></el-col>
          <el-col :span="8"><el-form-item :label="fat('syncFrequency')"><el-select v-model="form.syncFrequency" style="width: 100%"><el-option v-for="item in frequencies" :key="item[0]" :label="item[1]" :value="item[0]" /></el-select></el-form-item></el-col>
          <el-col :span="8"><el-form-item :label="fat('autoSync')"><el-switch v-model="form.syncEnabled" /></el-form-item></el-col>
          <el-col :span="24"><el-form-item :label="fat('description')"><el-input v-model="form.description" type="textarea" :rows="2" /></el-form-item></el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="test(form)">{{ fat('validateConfig') }}</el-button>
        <el-button @click="visible = false">{{ fat('cancel') }}</el-button>
        <el-button type="primary" :loading="saving" @click="save">{{ fat('save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>
