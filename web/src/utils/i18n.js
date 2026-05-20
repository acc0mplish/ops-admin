import { computed, ref } from 'vue'

const LOCALE_KEY = 'ops-admin-locale'

const messages = {
  'zh-CN': {
    language: '中文',
    english: 'English',
    dashboard: '仪表盘',
    profile: '个人信息',
    system: '系统管理',
    logs: '操作审计',
    admin: '用户信息',
    role: '角色信息',
    menu: '菜单信息',
    dept: '部门信息',
    post: '岗位信息',
    basicConfig: '基础配置',
    loginLog: '登录日志',
    operationLog: '操作日志',
    userCenter: '个人中心',
    logout: '退出登录',
    editProfile: '编辑资料',
    changePassword: '修改密码',
    accountInfo: '账户信息',
    extraInfo: '补充说明',
    saveConfig: '保存配置',
    configTitle: '基础配置',
    configDesc: '管理站点名称、登录文案、Logo 和界面主色。',
    siteName: '站点名称',
    siteSlogan: '站点标语',
    logoType: 'Logo 方式',
    logoValue: 'Logo 内容',
    loginTitle: '登录标题',
    loginSubtitle: '登录副标题',
    useLoginBackground: '启用背景图',
    loginBackground: '背景图 URL',
    primaryColor: '主色',
    sidebarTheme: '侧边栏风格',
    darkTheme: '深色',
    lightTheme: '浅色',
    textLogo: '文字 Logo',
    iconStyle: '图标风格',
    uploadLogo: '本地上传',
    imageUrl: '图片 URL',
    chooseImage: '选择图片',
    uploadBackground: '上传背景图',
    logoPreview: 'Logo 预览',
    loginPreview: '登录页预览',
    welcomeBack: '欢迎回来，{name}',
    dashboardDesc: '当前项目已经具备 system 域基础能力，可以作为你的个人运维平台核心控制台继续扩展。',
    userCount: '用户总数',
    roleCount: '角色总数',
    deptCount: '部门总数',
    postCount: '岗位总数',
    userCountNote: '系统账号数量',
    roleCountNote: '权限角色数量',
    deptCountNote: '组织结构节点',
    postCountNote: '岗位定义数量',
    loginWelcome: '欢迎登录',
    loginHint: '请输入账号和密码进入控制台。',
    loginSystem: '登录系统',
    username: '用户名',
    password: '密码',
    nickname: '昵称',
    email: '邮箱',
    phone: '手机号',
    note: '备注',
    confirmPassword: '确认密码',
    currentPassword: '当前密码',
    newPassword: '新密码',
    cancel: '取消',
    save: '保存',
    confirmChange: '确认修改',
    saveSuccess: '保存成功',
    profileUpdateSuccess: '个人信息已更新',
    passwordUpdateSuccess: '密码修改成功',
    switchLanguage: '国际化',
    superAdmin: '超级管理员',
    headquarters: '总部',
    noDept: '未设置部门',
    noPost: '未设置岗位',
    appSwitch: '应用切换',
    appConsole: '控制台',
    appAssets: '资产管理',
    appOps: '标准运维',
    appMonitor: '监控中心',
    assetsOverview: '资产概览',
    assetsHosts: '主机管理',
    assetsTags: '标签分组',
    opsOverview: '运维总览',
    opsJobs: '作业中心',
    opsScripts: '脚本执行',
    opsReleases: '发布编排',
    monitorOverview: '监控大盘',
    monitorAlerts: '告警中心',
    monitorMetrics: '指标看板',
    moduleReady: '模块入口已准备完成，可以继续在这里接入真实业务能力。'
  },
  'en-US': {
    language: 'Chinese',
    english: 'English',
    dashboard: 'Dashboard',
    profile: 'Profile',
    system: 'System',
    logs: 'Audit',
    admin: 'Users',
    role: 'Roles',
    menu: 'Menus',
    dept: 'Departments',
    post: 'Posts',
    basicConfig: 'Basic Config',
    loginLog: 'Login Log',
    operationLog: 'Operation Log',
    userCenter: 'Profile',
    logout: 'Logout',
    editProfile: 'Edit Profile',
    changePassword: 'Change Password',
    accountInfo: 'Account Info',
    extraInfo: 'Notes',
    saveConfig: 'Save Config',
    configTitle: 'Basic Config',
    configDesc: 'Manage site title, login copy, logo, and brand color.',
    siteName: 'Site Name',
    siteSlogan: 'Site Slogan',
    logoType: 'Logo Type',
    logoValue: 'Logo Value',
    loginTitle: 'Login Title',
    loginSubtitle: 'Login Subtitle',
    useLoginBackground: 'Use Background',
    loginBackground: 'Background URL',
    primaryColor: 'Primary Color',
    sidebarTheme: 'Sidebar Theme',
    darkTheme: 'Dark',
    lightTheme: 'Light',
    textLogo: 'Text Logo',
    iconStyle: 'Icon Style',
    uploadLogo: 'Upload',
    imageUrl: 'Image URL',
    chooseImage: 'Choose Image',
    uploadBackground: 'Upload Background',
    logoPreview: 'Logo Preview',
    loginPreview: 'Login Preview',
    welcomeBack: 'Welcome back, {name}',
    dashboardDesc: 'The system domain is ready and can keep growing into your personal operations platform.',
    userCount: 'Users',
    roleCount: 'Roles',
    deptCount: 'Departments',
    postCount: 'Posts',
    userCountNote: 'System accounts',
    roleCountNote: 'Permission roles',
    deptCountNote: 'Organization nodes',
    postCountNote: 'Defined positions',
    loginWelcome: 'Welcome',
    loginHint: 'Enter your credentials to access the console.',
    loginSystem: 'Sign In',
    username: 'Username',
    password: 'Password',
    nickname: 'Nickname',
    email: 'Email',
    phone: 'Phone',
    note: 'Note',
    confirmPassword: 'Confirm Password',
    currentPassword: 'Current Password',
    newPassword: 'New Password',
    cancel: 'Cancel',
    save: 'Save',
    confirmChange: 'Confirm',
    saveSuccess: 'Saved successfully',
    profileUpdateSuccess: 'Profile updated',
    passwordUpdateSuccess: 'Password updated',
    switchLanguage: 'Language',
    superAdmin: 'Super Admin',
    headquarters: 'Headquarters',
    noDept: 'No department',
    noPost: 'No post',
    appSwitch: 'Applications',
    appConsole: 'Console',
    appAssets: 'Asset Management',
    appOps: 'Standard Operations',
    appMonitor: 'Monitoring Center',
    assetsOverview: 'Asset Overview',
    assetsHosts: 'Host Management',
    assetsTags: 'Tags',
    opsOverview: 'Operations Overview',
    opsJobs: 'Job Center',
    opsScripts: 'Scripts',
    opsReleases: 'Release Orchestration',
    monitorOverview: 'Monitoring Dashboard',
    monitorAlerts: 'Alert Center',
    monitorMetrics: 'Metrics Board',
    moduleReady: 'This module entry is ready for your real business capabilities.'
  }
}

export const locale = ref(localStorage.getItem(LOCALE_KEY) || 'zh-CN')

export function setLocale(value) {
  locale.value = value || 'zh-CN'
  localStorage.setItem(LOCALE_KEY, locale.value)
}

export function getLocale() {
  return locale.value
}

export function t(key, params = {}) {
  const dict = messages[locale.value] || messages['zh-CN']
  let text = dict[key] || messages['zh-CN'][key] || key
  Object.keys(params).forEach((name) => {
    text = text.replace(`{${name}}`, params[name])
  })
  return text
}

const routeTitleMap = {
  '/dashboard': 'dashboard',
  '/profile': 'profile',
  '/system/admin': 'admin',
  '/system/role': 'role',
  '/system/menu': 'menu',
  '/system/dept': 'dept',
  '/system/post': 'post',
  '/system/basic-config': 'basicConfig',
  '/logs/login': 'loginLog',
  '/logs/operation': 'operationLog',
  '/system': 'system',
  '/logs': 'logs',
  '/assets/overview': 'assetsOverview',
  '/assets/hosts': 'assetsHosts',
  '/assets/tags': 'assetsTags',
  '/ops/overview': 'opsOverview',
  '/ops/jobs': 'opsJobs',
  '/ops/scripts': 'opsScripts',
  '/ops/releases': 'opsReleases',
  '/monitor/overview': 'monitorOverview',
  '/monitor/alerts': 'monitorAlerts',
  '/monitor/metrics': 'monitorMetrics'
}

export function translateRoute(path, fallback = '') {
  const key = routeTitleMap[path]
  return key ? t(key) : fallback
}

export function translateEntity(value, fallback = '-') {
  const map = {
    'Super Admin': 'superAdmin',
    '超级管理员': 'superAdmin',
    Headquarters: 'headquarters',
    总部: 'headquarters'
  }
  return map[value] ? t(map[value]) : value || fallback
}

export function useI18nState() {
  return {
    locale: computed(() => locale.value),
    t
  }
}
