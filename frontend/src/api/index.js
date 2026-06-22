import axios from 'axios'
import { ElMessage } from 'element-plus'

const getApiKey = () => localStorage.getItem('abac_api_key') || ''
const getAdminToken = () => localStorage.getItem('abac_admin_token') || ''

const api = axios.create({
  baseURL: '/api/v1',
  timeout: 15000
})

api.interceptors.request.use(config => {
  const apiKey = getApiKey()
  const adminToken = getAdminToken()
  if (apiKey) {
    config.headers['X-API-Key'] = apiKey
  }
  if (adminToken && config.url.startsWith('/admin')) {
    config.headers['X-Admin-Token'] = adminToken
  }
  return config
})

api.interceptors.response.use(
  response => response.data,
  error => {
    const msg = error.response?.data?.error || error.message || '请求失败'
    ElMessage.error(msg)
    return Promise.reject(error)
  }
)

export const healthCheck = () => api.get('/health')
export const getAttributes = () => api.get('/attributes')

export const decide = (data) => api.post('/decide', data)
export const decideTrace = (data) => api.post('/decide/trace', data)
export const simulate = (data) => api.post('/simulate', data)
export const simulateBatch = (data) => api.post('/simulate/batch', data)

export const listPolicies = () => api.get('/policies')
export const getPolicy = (id) => api.get(`/policies/${id}`)
export const createPolicy = (data) => api.post('/policies', data)
export const updatePolicy = (id, data) => api.put(`/policies/${id}`, data)
export const deletePolicy = (id) => api.delete(`/policies/${id}`)
export const validatePolicy = (yaml) => api.post('/policies/validate', { yaml })
export const togglePolicy = (id, status) => api.post(`/policies/${id}/status`, { status })
export const getDependencyGraph = () => api.get('/policies/dependency/graph')

export const listVersions = (policyId) => api.get(`/policies/${policyId}/versions`)
export const rollbackPolicy = (policyId, data) => api.post(`/policies/${policyId}/rollback`, data)

export const queryAudit = (params) => api.get('/audit', { params })
export const exportAudit = (params) => {
  const apiKey = getApiKey()
  const queryStr = new URLSearchParams(params).toString()
  window.open(`/api/v1/audit/export?${queryStr}&api_key=${apiKey}`, '_blank')
}

export const listTenants = (params) => api.get('/admin/tenants', { params })
export const createTenant = (data) => api.post('/admin/tenants', data)
export const updateTenant = (id, data) => api.put(`/admin/tenants/${id}`, data)
export const deleteTenant = (id) => api.delete(`/admin/tenants/${id}`)

export default api
