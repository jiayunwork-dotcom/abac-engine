<template>
  <el-container class="layout-container">
    <el-aside :width="sidebarWidth" class="sidebar">
      <div class="logo">
        <el-icon :size="28"><Lock /></el-icon>
        <span v-show="!store.sidebarCollapsed" class="logo-text">ABAC 控制台</span>
      </div>
      <el-menu
        :default-active="$route.path"
        :collapse="store.sidebarCollapsed"
        router
        background-color="#1f2937"
        text-color="#9ca3af"
        active-text-color="#60a5fa"
      >
        <template v-for="route in menuRoutes" :key="route.path">
          <el-menu-item v-if="!route.meta?.admin || isAdminMode" :index="route.path">
            <el-icon><component :is="route.meta.icon" /></el-icon>
            <template #title>{{ route.meta.title }}</template>
          </el-menu-item>
        </template>
      </el-menu>
    </el-aside>

    <el-container>
      <el-header class="header">
        <div class="header-left">
          <el-button text @click="store.toggleSidebar()">
            <el-icon :size="20"><Fold v-if="!store.sidebarCollapsed" /><Expand v-else /></el-icon>
          </el-button>
          <el-breadcrumb separator="/">
            <el-breadcrumb-item :to="{ path: '/' }">首页</el-breadcrumb-item>
            <el-breadcrumb-item>{{ $route.meta.title }}</el-breadcrumb-item>
          </el-breadcrumb>
        </div>
        <div class="header-right">
          <el-tag :type="isAdminMode ? 'warning' : 'success'" size="small" class="mode-tag">
            {{ isAdminMode ? '平台管理员' : '租户模式' }}
          </el-tag>
          <el-tooltip content="切换模式" placement="bottom">
            <el-switch
              v-model="isAdminMode"
              active-text="管理员"
              inactive-text="租户"
              inline-prompt
              style="--el-switch-on-color: #f59e0b; --el-switch-off-color: #10b981"
            />
          </el-tooltip>
        </div>
      </el-header>

      <el-main class="main-content">
        <router-view v-slot="{ Component }">
          <transition name="fade" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup>
import { computed, ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useAppStore } from '@/stores/app'
import { getAttributes } from '@/api'

const store = useAppStore()
const route = useRoute()

const menuRoutes = computed(() => {
  return [
    { path: '/policies', meta: { title: '策略列表', icon: 'List' } },
    { path: '/simulate', meta: { title: '决策测试台', icon: 'Cpu' } },
    { path: '/audit', meta: { title: '审计查询', icon: 'Document' } },
    { path: '/tenants', meta: { title: '租户管理', icon: 'OfficeBuilding', admin: true } },
    { path: '/settings', meta: { title: '系统设置', icon: 'Setting' } }
  ]
})

const sidebarWidth = computed(() => store.sidebarCollapsed ? '64px' : '220px')
const isAdminMode = ref(localStorage.getItem('abac_admin_mode') === 'true')

onMounted(async () => {
  try {
    const data = await getAttributes()
    store.setAttributes(data)
  } catch (e) {
    console.warn('Failed to load attributes')
  }
})

isAdminMode.value = localStorage.getItem('abac_admin_mode') === 'true'
</script>

<script>
export default {
  watch: {
    isAdminMode(val) {
      localStorage.setItem('abac_admin_mode', val ? 'true' : 'false')
    }
  }
}
</script>

<style scoped>
.layout-container {
  min-height: 100vh;
}
.sidebar {
  background: #1f2937;
  transition: width 0.3s;
  overflow: hidden;
}
.logo {
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  color: #fff;
  border-bottom: 1px solid #374151;
  padding: 0 16px;
}
.logo-text {
  font-size: 16px;
  font-weight: 600;
  white-space: nowrap;
}
.sidebar :deep(.el-menu) {
  border-right: none;
}
.header {
  background: #fff;
  border-bottom: 1px solid #e5e7eb;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
}
.header-left {
  display: flex;
  align-items: center;
  gap: 16px;
}
.header-right {
  display: flex;
  align-items: center;
  gap: 16px;
}
.mode-tag {
  margin-right: 4px;
}
.main-content {
  padding: 0;
  background: #f0f2f5;
}
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
