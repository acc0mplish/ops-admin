<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { deleteDNSAccount, getDNSAccount, queryDNSAccounts, saveDNSAccount, testDNSAccount } from '../../api/domain'
import { dat } from '../../utils/dns-account-i18n'

const loading = ref(false), saving = ref(false), dialogVisible = ref(false)
const testingId = ref(0), tableData = ref([]), total = ref(0)
const query = reactive({ pageNum: 1, pageSize: 10, keyword: '', provider: '' })
const form = reactive({ id: 0, name: '', provider: 'aliyun', accessKey: '', secretKey: '', status: 1 })

async function loadData(){loading.value=true;try{const data=await queryDNSAccounts(query);tableData.value=data.list||[];total.value=data.total||0}finally{loading.value=false}}
function openCreate(){Object.assign(form,{id:0,name:'',provider:'aliyun',accessKey:'',secretKey:'',status:1});dialogVisible.value=true}
async function openEdit(row){const data=await getDNSAccount(row.id);Object.assign(form,data,{accessKey:'',secretKey:''});dialogVisible.value=true}
async function submit(){if(!form.name.trim()){ElMessage.warning(dat('accountNameRequired'));return}if(!form.id&&(!form.accessKey.trim()||!form.secretKey.trim())){ElMessage.warning(dat('credentialsRequired'));return}saving.value=true;try{await saveDNSAccount(form);ElMessage.success(form.id?dat('updated'):dat('created'));dialogVisible.value=false;await loadData()}finally{saving.value=false}}
async function test(row){testingId.value=row.id;try{await testDNSAccount(row.id);ElMessage.success(dat('connectionSucceeded'));await loadData()}finally{testingId.value=0}}
async function remove(row){await ElMessageBox.confirm(dat('deleteConfirm',{name:row.name}),dat('deleteTitle'),{type:'warning'});await deleteDNSAccount(row.id);ElMessage.success(dat('deleted'));await loadData()}
onMounted(loadData)
</script>

<template>
  <section class="domain-page domain-panel page-card">
    <div class="domain-page-head">
      <div><div class="domain-eyebrow">PUBLIC DNS CREDENTIALS</div><h2>{{ dat('title') }}</h2><p>{{ dat('description') }}</p></div>
      <el-button v-permission="'domains:account:add'" type="primary" @click="openCreate">{{ dat('add') }}</el-button>
    </div>
    <div class="domain-toolbar" role="search">
      <div class="domain-toolbar__filters"><el-input v-model="query.keyword" clearable :placeholder="dat('searchName')" style="width:240px" @keyup.enter="loadData"/><el-select v-model="query.provider" clearable :placeholder="dat('provider')" style="width:160px"><el-option :label="dat('aliyun')" value="aliyun"/><el-option :label="dat('tencent')" value="tencent"/></el-select><el-button @click="loadData">{{ dat('search') }}</el-button></div>
    </div>
    <div class="domain-table-wrap">
      <el-table v-loading="loading" :data="tableData" border>
        <el-table-column prop="name" :label="dat('accountName')" min-width="170"/><el-table-column :label="dat('provider')" width="150"><template #default="{row}">{{row.provider==='aliyun'?dat('aliyun'):dat('tencent')}}</template></el-table-column>
        <el-table-column prop="accessKeyHint" :label="dat('credentialHint')" min-width="160"><template #default="{row}"><span class="domain-mono">{{row.accessKeyHint}}</span></template></el-table-column>
        <el-table-column :label="dat('connectionStatus')" width="140"><template #default="{row}"><span class="dns-state" :class="{'is-running':row.lastConnectionStatus==='success','is-error':row.lastConnectionStatus==='failed'}">{{row.lastConnectionStatus==='success'?dat('healthy'):row.lastConnectionStatus==='failed'?dat('failed'):dat('notTested')}}</span></template></el-table-column>
        <el-table-column :label="dat('accountStatus')" width="100"><template #default="{row}"><el-tag :type="row.status===1?'success':'info'">{{row.status===1?dat('enable'):dat('disable')}}</el-tag></template></el-table-column>
        <el-table-column prop="lastConnectionAt" :label="dat('lastTest')" width="180"/><el-table-column :label="dat('actions')" width="210" fixed="right"><template #default="{row}"><el-button v-permission="'domains:account:test'" link type="primary" :loading="testingId===row.id" @click="test(row)">{{ dat('testConnection') }}</el-button><el-button v-permission="'domains:account:edit'" link @click="openEdit(row)">{{ dat('edit') }}</el-button><el-button v-permission="'domains:account:delete'" link type="danger" @click="remove(row)">{{ dat('delete') }}</el-button></template></el-table-column>
        <template #empty><div class="domain-empty"><strong>{{ dat('noAccounts') }}</strong><p>{{ dat('noAccountsDesc') }}</p><el-button v-permission="'domains:account:add'" type="primary" @click="openCreate">{{ dat('add') }}</el-button></div></template>
      </el-table>
    </div>
    <div class="domain-pager"><el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" layout="total, sizes, prev, pager, next" :total="total" @current-change="loadData" @size-change="loadData"/></div>
    <el-dialog v-model="dialogVisible" :title="form.id?dat('editTitle'):dat('addTitle')" width="min(600px, calc(100vw - 32px))">
      <el-form label-width="110px"><el-form-item :label="dat('accountName')" required><el-input v-model="form.name"/></el-form-item><el-form-item :label="dat('dnsProvider')" required><el-select v-model="form.provider" style="width:100%"><el-option :label="dat('aliyun')" value="aliyun"/><el-option :label="dat('tencent')" value="tencent"/></el-select></el-form-item><el-form-item :label="form.provider==='aliyun'?'AccessKey':'SecretId'" :required="!form.id"><el-input v-model="form.accessKey" autocomplete="off" :placeholder="form.id?dat('leaveBlank'):''"/><div class="domain-form-tip">{{ dat('credentialSecurity') }}</div></el-form-item><el-form-item :label="form.provider==='aliyun'?'AccessSecret':'SecretKey'" :required="!form.id"><el-input v-model="form.secretKey" type="password" show-password autocomplete="new-password" :placeholder="form.id?dat('leaveBlank'):''"/></el-form-item><el-form-item :label="dat('status')"><el-switch v-model="form.status" :active-value="1" :inactive-value="2" :active-text="dat('enable')" :inactive-text="dat('disable')"/></el-form-item></el-form>
      <template #footer><el-button @click="dialogVisible=false">{{ dat('cancel') }}</el-button><el-button type="primary" :loading="saving" @click="submit">{{ dat('saveAccount') }}</el-button></template>
    </el-dialog>
  </section>
</template>
