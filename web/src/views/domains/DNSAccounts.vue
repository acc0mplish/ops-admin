<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { deleteDNSAccount, getDNSAccount, queryDNSAccounts, saveDNSAccount, testDNSAccount } from '../../api/domain'

const loading = ref(false), saving = ref(false), dialogVisible = ref(false)
const testingId = ref(0), tableData = ref([]), total = ref(0)
const query = reactive({ pageNum: 1, pageSize: 10, keyword: '', provider: '' })
const form = reactive({ id: 0, name: '', provider: 'aliyun', accessKey: '', secretKey: '', status: 1 })

async function loadData(){loading.value=true;try{const data=await queryDNSAccounts(query);tableData.value=data.list||[];total.value=data.total||0}finally{loading.value=false}}
function openCreate(){Object.assign(form,{id:0,name:'',provider:'aliyun',accessKey:'',secretKey:'',status:1});dialogVisible.value=true}
async function openEdit(row){const data=await getDNSAccount(row.id);Object.assign(form,data,{accessKey:'',secretKey:''});dialogVisible.value=true}
async function submit(){if(!form.name.trim()){ElMessage.warning('请输入账号名称');return}if(!form.id&&(!form.accessKey.trim()||!form.secretKey.trim())){ElMessage.warning('请填写完整的云厂商凭据');return}saving.value=true;try{await saveDNSAccount(form);ElMessage.success(form.id?'DNS 账号已更新':'DNS 账号已创建');dialogVisible.value=false;await loadData()}finally{saving.value=false}}
async function test(row){testingId.value=row.id;try{await testDNSAccount(row.id);ElMessage.success('连接成功');await loadData()}finally{testingId.value=0}}
async function remove(row){await ElMessageBox.confirm(`删除账号“${row.name}”后将同时移除域名同步快照，确认继续？`,'删除 DNS 账号',{type:'warning'});await deleteDNSAccount(row.id);ElMessage.success('DNS 账号已删除');await loadData()}
onMounted(loadData)
</script>

<template>
  <section class="domain-page domain-panel page-card">
    <div class="domain-page-head">
      <div><div class="domain-eyebrow">PUBLIC DNS CREDENTIALS</div><h2>公网 DNS 账号</h2><p>接入阿里云 DNS 与腾讯云 DNSPod。凭据加密保存，页面仅展示脱敏标识。</p></div>
      <el-button v-permission="'domains:account:add'" type="primary" @click="openCreate">新增 DNS 账号</el-button>
    </div>
    <div class="domain-toolbar" role="search">
      <div class="domain-toolbar__filters"><el-input v-model="query.keyword" clearable placeholder="搜索账号名称" style="width:240px" @keyup.enter="loadData"/><el-select v-model="query.provider" clearable placeholder="服务商" style="width:160px"><el-option label="阿里云 DNS" value="aliyun"/><el-option label="腾讯云 DNSPod" value="tencent"/></el-select><el-button @click="loadData">查询</el-button></div>
    </div>
    <div class="domain-table-wrap">
      <el-table v-loading="loading" :data="tableData" border>
        <el-table-column prop="name" label="账号名称" min-width="170"/><el-table-column label="服务商" width="150"><template #default="{row}">{{row.provider==='aliyun'?'阿里云 DNS':'腾讯云 DNSPod'}}</template></el-table-column>
        <el-table-column prop="accessKeyHint" label="凭据标识" min-width="160"><template #default="{row}"><span class="domain-mono">{{row.accessKeyHint}}</span></template></el-table-column>
        <el-table-column label="连接状态" width="140"><template #default="{row}"><span class="dns-state" :class="{'is-running':row.lastConnectionStatus==='success','is-error':row.lastConnectionStatus==='failed'}">{{row.lastConnectionStatus==='success'?'连接正常':row.lastConnectionStatus==='failed'?'连接失败':'尚未测试'}}</span></template></el-table-column>
        <el-table-column label="账号状态" width="100"><template #default="{row}"><el-tag :type="row.status===1?'success':'info'">{{row.status===1?'启用':'停用'}}</el-tag></template></el-table-column>
        <el-table-column prop="lastConnectionAt" label="最近测试" width="180"/><el-table-column label="操作" width="210" fixed="right"><template #default="{row}"><el-button v-permission="'domains:account:test'" link type="primary" :loading="testingId===row.id" @click="test(row)">测试连接</el-button><el-button v-permission="'domains:account:edit'" link @click="openEdit(row)">编辑</el-button><el-button v-permission="'domains:account:delete'" link type="danger" @click="remove(row)">删除</el-button></template></el-table-column>
        <template #empty><div class="domain-empty"><strong>尚未配置公网 DNS 账号</strong><p>添加云厂商账号后才能同步公网域名。</p><el-button v-permission="'domains:account:add'" type="primary" @click="openCreate">新增 DNS 账号</el-button></div></template>
      </el-table>
    </div>
    <div class="domain-pager"><el-pagination v-model:current-page="query.pageNum" v-model:page-size="query.pageSize" layout="total, sizes, prev, pager, next" :total="total" @current-change="loadData" @size-change="loadData"/></div>
    <el-dialog v-model="dialogVisible" :title="form.id?'编辑 DNS 账号':'新增 DNS 账号'" width="min(600px, calc(100vw - 32px))">
      <el-form label-width="110px"><el-form-item label="账号名称" required><el-input v-model="form.name"/></el-form-item><el-form-item label="DNS 服务商" required><el-select v-model="form.provider" style="width:100%"><el-option label="阿里云 DNS" value="aliyun"/><el-option label="腾讯云 DNSPod" value="tencent"/></el-select></el-form-item><el-form-item :label="form.provider==='aliyun'?'AccessKey':'SecretId'" :required="!form.id"><el-input v-model="form.accessKey" autocomplete="off" :placeholder="form.id?'留空保持不变':''"/><div class="domain-form-tip">凭据使用 AES-GCM 加密保存，列表和详情接口不会返回原值。</div></el-form-item><el-form-item :label="form.provider==='aliyun'?'AccessSecret':'SecretKey'" :required="!form.id"><el-input v-model="form.secretKey" type="password" show-password autocomplete="new-password" :placeholder="form.id?'留空保持不变':''"/></el-form-item><el-form-item label="状态"><el-switch v-model="form.status" :active-value="1" :inactive-value="2" active-text="启用" inactive-text="停用"/></el-form-item></el-form>
      <template #footer><el-button @click="dialogVisible=false">取消</el-button><el-button type="primary" :loading="saving" @click="submit">保存账号</el-button></template>
    </el-dialog>
  </section>
</template>
