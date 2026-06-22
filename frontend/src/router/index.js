import { createRouter, createWebHashHistory } from 'vue-router'

const routes = [
  {
    path: '/',
    redirect: '/policies'
  },
  {
    path: '/policies',
    name: 'Policies',
    component: () => import('@/views/PolicyList.vue'),
    meta: { title: '策略列表', icon: 'List' }
  },
  {
    path: '/policies/new',
    name: 'PolicyNew',
    component: () => import('@/views/PolicyEditor.vue'),
    meta: { title: '新建策略', icon: 'Plus', hidden: true }
  },
  {
    path: '/policies/:id/edit',
    name: 'PolicyEdit',
    component: () => import('@/views/PolicyEditor.vue'),
    meta: { title: '编辑策略', icon: 'Edit', hidden: true }
  },
  {
    path: '/policies/:id/versions',
    name: 'PolicyVersions',
    component: () => import('@/views/VersionHistory.vue'),
    meta: { title: '版本历史', icon: 'Clock', hidden: true }
  },
  {
    path: '/simulate',
    name: 'Simulate',
    component: () => import('@/views/DecisionTest.vue'),
    meta: { title: '决策测试台', icon: 'Cpu' }
  },
  {
    path: '/audit',
    name: 'Audit',
    component: () => import('@/views/AuditLog.vue'),
    meta: { title: '审计查询', icon: 'Document' }
  },
  {
    path: '/tenants',
    name: 'Tenants',
    component: () => import('@/views/TenantManagement.vue'),
    meta: { title: '租户管理', icon: 'OfficeBuilding', admin: true }
  },
  {
    path: '/settings',
    name: 'Settings',
    component: () => import('@/views/Settings.vue'),
    meta: { title: '系统设置', icon: 'Setting' }
  }
]

const router = createRouter({
  history: createWebHashHistory(),
  routes
})

router.afterEach((to) => {
  document.title = (to.meta.title ? to.meta.title + ' - ' : '') + 'ABAC 策略管理控制台'
})

export default router
