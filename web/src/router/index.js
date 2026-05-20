import { createRouter, createWebHistory } from 'vue-router'

import MainLayout from '../layouts/MainLayout.vue'
import Login from '../views/Login.vue'
import Dashboard from '../views/Dashboard.vue'
import Profile from '../views/Profile.vue'
import Admin from '../views/system/Admin.vue'
import Role from '../views/system/Role.vue'
import Menu from '../views/system/Menu.vue'
import Dept from '../views/system/Dept.vue'
import Post from '../views/system/Post.vue'
import LoginLog from '../views/system/LoginLog.vue'
import OperationLog from '../views/system/OperationLog.vue'
import BasicConfig from '../views/system/BasicConfig.vue'
import { getToken } from '../utils/auth'

const routes = [
  { path: '/login', component: Login, meta: { title: '登录' } },
  {
    path: '/',
    component: MainLayout,
    redirect: '/dashboard',
    children: [
      { path: '/dashboard', component: Dashboard, meta: { title: '仪表盘', affix: true } },
      { path: '/profile', component: Profile, meta: { title: '个人信息' } },
      { path: '/system/admin', component: Admin, meta: { title: '用户信息' } },
      { path: '/system/role', component: Role, meta: { title: '角色信息' } },
      { path: '/system/menu', component: Menu, meta: { title: '菜单信息' } },
      { path: '/system/dept', component: Dept, meta: { title: '部门信息' } },
      { path: '/system/post', component: Post, meta: { title: '岗位信息' } },
      { path: '/system/basic-config', component: BasicConfig, meta: { title: '基础配置' } },
      { path: '/logs/login', component: LoginLog, meta: { title: '登录日志' } },
      { path: '/logs/operation', component: OperationLog, meta: { title: '操作日志' } }
    ]
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach((to, from, next) => {
  if (to.path === '/login') {
    if (getToken()) {
      next('/dashboard')
      return
    }
    next()
    return
  }

  if (!getToken()) {
    next('/login')
    return
  }
  next()
})

export default router
