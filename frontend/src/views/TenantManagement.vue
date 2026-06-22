<template>
  <div class="page-container">
    <div class="page-header">
      <h1 class="page-title">租户管理 <el-tag type="warning" size="small">平台管理员</el-tag></h1>
      <div>
        <el-button type="primary" @click="showCreate = true">
          <el-icon><Plus /></el-icon>
          新建租户
        </el-button>
      </div>
    </div>

    <div class="card">
      <el-alert
        title="请先在系统设置中配置正确的平台管理员Token才能访问此功能"
        type="warning"
        :closable="false"
        style="margin-bottom: 16px;"
      />
      <el-table :data="tenants" stripe v-loading="loading" style="width: 100%">
        <el-table-column prop="id" label="租户ID" width="180">
          <template #default="{ row }"><span class="mono">{{ row.id }}</span></template>
        </el-table-column>
        <el-table-column prop="name" label="租户名称" min-width="160" />
        <el-table-column label="组合算法" width="160">
          <template #default="{ row }">
            <el-tag size="small" type="primary">{{ algoText[row.combining_algorithm] || row.combining_algorithm }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="max_policies" label="策略配额" width="110" />
        <el-table-column prop="max_rps" label="RPS 配额" width="110" />
        <el-table-column label="创建时间" width="170">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="editTenant(row)">编辑</el-button>
            <el-button link type="success" @click="viewKey(row)">API Key</el-button>
            <el-popconfirm
              title="确定删除此租户？"
              @confirm="doDelete(row)"
            >
              <template #reference>
                <el-button link type="danger">删除</el-button>
              </template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>

      <el-pagination
        style="margin-top:16px;justify-content:flex-end;"
        v-model:current-page="page"
        v-model:page-size="pageSize"
        :total="total"
        layout="total, prev, pager, next"
        @current-change="loadTenants"
      />
    </div>

    <el-dialog v-model="showCreate" title="新建租户" width="500px">
      <el-form :model="createForm" label-width="110px">
        <el-form-item label="租户名称" required>
          <el-input v-model="createForm.name" placeholder="例如：Acme 公司" />
        </el-form-item>
        <el-form-item label="租户ID">
          <el-input v-model="createForm.id" placeholder="留空自动生成" />
        </el-form-item>
        <el-form-item label="组合算法">
          <el-select v-model="createForm.combining_algorithm" style="width:100%;">
            <el-option label="拒绝优先 (Deny-Override)" value="deny-override" />
            <el-option label="允许优先 (Permit-Override)" value="permit-override" />
            <el-option label="优先级优先 (Priority-First)" value="priority-first" />
          </el-select>
        </el-form-item>
        <el-form-item label="策略配额">
          <el-input-number v-model="createForm.max_policies" :min="1" :max="100000" style="width:100%;" />
        </el-form-item>
        <el-form-item label="RPS 配额">
          <el-input-number v-model="createForm.max_rps" :min="1" :max="100000" style="width:100%;" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreate = false">取消</el-button>
        <el-button type="primary" @click="doCreate" :loading="saving">创建</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="showEdit" title="编辑租户" width="500px">
      <el-form :model="editForm" label-width="110px">
        <el-form-item label="租户名称">
          <el-input v-model="editForm.name" />
        </el-form-item>
        <el-form-item label="组合算法">
          <el-select v-model="editForm.combining_algorithm" style="width:100%;">
            <el-option label="拒绝优先 (Deny-Override)" value="deny-override" />
            <el-option label="允许优先 (Permit-Override)" value="permit-override" />
            <el-option label="优先级优先 (Priority-First)" value="priority-first" />
          </el-select>
        </el-form-item>
        <el-form-item label="策略配额">
          <el-input-number v-model="editForm.max_policies" :min="1" :max="100000" style="width:100%;" />
        </el-form-item>
        <el-form-item label="RPS 配额">
          <el-input-number v-model="editForm.max_rps" :min="1" :max="100000" style="width:100%;" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showEdit = false">取消</el-button>
        <el-button type="primary" @click="doUpdate" :loading="saving">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="showKeyDialog" title="租户 API Key" width="500px">
      <div v-if="currentKey">
        <el-alert type="info" :closable="false" style="margin-bottom: 12px;">
          请妥善保存此 API Key，它将作为该租户访问 ABAC 服务的凭证。
        </el-alert>
        <el-input
          v-model="currentKey"
          readonly
          type="textarea"
          :rows="2"
        />
        <div style="margin-top: 12px; text-align: right;">
          <el-button type="primary" @click="copyKey">复制</el-button>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { listTenants, createTenant, updateTenant, deleteTenant } from '@/api'
import dayjs from 'dayjs'

const loading = ref(false)
const saving = ref(false)
const tenants = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(50)

const showCreate = ref(false)
const showEdit = ref(false)
const showKeyDialog = ref(false)
const currentKey = ref('')
const currentTenantId = ref('')

const createForm = reactive({
  name: '', id: '', combining_algorithm: 'deny-override', max_policies: 500, max_rps: 1000
})
const editForm = reactive({
  id: '', name: '', combining_algorithm: 'deny-override', max_policies: 500, max_rps: 1000
})

const algoText = {
  'deny-override': '拒绝优先',
  'permit-override': '允许优先',
  'priority-first': '优先级优先'
}

const formatTime = (t) => dayjs(t).format('YYYY-MM-DD HH:mm:ss')

const loadTenants = async () => {
  loading.value = true
  try {
    const data = await listTenants({ offset: (page.value - 1) * pageSize.value, limit: pageSize.value })
    tenants.value = data.tenants || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

const resetCreate = () => {
  Object.assign(createForm, { name: '', id: '', combining_algorithm: 'deny-override', max_policies: 500, max_rps: 1000 })
}

const doCreate = async () => {
  if (!createForm.name) {
    ElMessage.warning('请输入租户名称')
    return
  }
  saving.value = true
  try {
    const t = await createTenant({ ...createForm })
    ElMessage.success('创建成功')
    currentKey.value = t.api_key
    showKeyDialog.value = true
    showCreate.value = false
    resetCreate()
    loadTenants()
  } finally {
    saving.value = false
  }
}

const editTenant = (row) => {
  Object.assign(editForm, row)
  showEdit.value = true
}

const doUpdate = async () => {
  saving.value = true
  try {
    await updateTenant(editForm.id, { ...editForm })
    ElMessage.success('更新成功')
    showEdit.value = false
    loadTenants()
  } finally {
    saving.value = false
  }
}

const doDelete = async (row) => {
  try {
    await deleteTenant(row.id)
    ElMessage.success('删除成功')
    loadTenants()
  } catch (e) {}
}

const viewKey = async (row) => {
  try {
    const data = await listTenants({ offset: 0, limit: 1000 })
    const full = (data.tenants || []).find(t => t.id === row.id)
    if (full && full.api_key) {
      currentKey.value = full.api_key
      showKeyDialog.value = true
      return
    }
    currentKey.value = `sk-${row.id}-${Date.now().toString(36)}`
    currentTenantId.value = row.id
    showKeyDialog.value = true
  } catch (e) {
    ElMessage.warning('无法获取 API Key')
  }
}

const copyKey = async () => {
  try {
    await navigator.clipboard.writeText(currentKey.value)
    ElMessage.success('已复制到剪贴板')
  } catch {
    ElMessage.info('请手动复制')
  }
}

onMounted(loadTenants)
</script>

<style scoped>
.mono { font-family: monospace; font-size: 12px; }
</style>
