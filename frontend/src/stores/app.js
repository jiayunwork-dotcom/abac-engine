import { defineStore } from 'pinia'

export const useAppStore = defineStore('app', {
  state: () => ({
    apiKey: localStorage.getItem('abac_api_key') || 'sk-demo-tenant-key-12345',
    adminToken: localStorage.getItem('abac_admin_token') || 'admin-secret-token-change-me',
    tenantId: localStorage.getItem('abac_tenant_id') || 'tenant-demo',
    attributes: null,
    sidebarCollapsed: false
  }),
  actions: {
    setApiKey(key) {
      this.apiKey = key
      localStorage.setItem('abac_api_key', key)
    },
    setAdminToken(token) {
      this.adminToken = token
      localStorage.setItem('abac_admin_token', token)
    },
    setTenantId(id) {
      this.tenantId = id
      localStorage.setItem('abac_tenant_id', id)
    },
    setAttributes(attrs) {
      this.attributes = attrs
    },
    toggleSidebar() {
      this.sidebarCollapsed = !this.sidebarCollapsed
    }
  }
})
