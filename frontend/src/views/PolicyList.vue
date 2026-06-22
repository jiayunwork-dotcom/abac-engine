<template>
  <div class="page-container">
    <div class="page-header">
      <h1 class="page-title">策略管理</h1>
      <div>
        <el-button type="primary" @click="goCreate">
          <el-icon><Plus /></el-icon>
          新建策略
        </el-button>
      </div>
    </div>

    <div class="card">
      <div class="filter-bar">
        <el-input
          v-model="searchKeyword"
          placeholder="搜索策略ID/描述"
          :prefix-icon="Search"
          clearable
          style="width: 280px"
        />
        <el-select v-model="filterLevel" placeholder="策略层级" clearable style="width: 140px">
          <el-option label="全局策略" value="global" />
          <el-option label="租户策略" value="tenant" />
          <el-option label="项目策略" value="project" />
        </el-select>
        <el-select v-model="filterEffect" placeholder="效果" clearable style="width: 120px">
          <el-option label="允许" value="permit" />
          <el-option label="拒绝" value="deny" />
        </el-select>
        <el-select v-model="filterStatus" placeholder="状态" clearable style="width: 120px">
          <el-option label="启用" value="enabled" />
          <el-option label="禁用" value="disabled" />
        </el-select>
        <el-button @click="loadPolicies">
          <el-icon><Refresh /></el-icon>
          刷新
        </el-button>
      </div>

      <el-table :data="filteredPolicies" stripe v-loading="loading" style="width: 100%">
        <el-table-column prop="id" label="策略ID" width="180" fixed>
          <template #default="{ row }">
            <span class="mono">{{ row.id }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="description" label="描述" min-width="200" show-overflow-tooltip />
        <el-table-column prop="level" label="层级" width="100">
          <template #default="{ row }">
            <el-tag :class="'tag-' + row.level" size="small">
              {{ levelText[row.level] }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="effect" label="效果" width="90">
          <template #default="{ row }">
            <el-tag :class="row.effect === 'permit' ? 'tag-permit' : 'tag-deny'" size="small">
              {{ row.effect === 'permit' ? '允许' : '拒绝' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="priority" label="优先级" width="80" sortable />
        <el-table-column prop="version" label="版本" width="70" />
        <el-table-column prop="status" label="状态" width="90">
          <template #default="{ row }">
            <el-switch
              v-model="row._status"
              @change="toggleStatus(row)"
              active-value="enabled"
              inactive-value="disabled"
            />
          </template>
        </el-table-column>
        <el-table-column prop="updated_at" label="更新时间" width="170">
          <template #default="{ row }">
            {{ formatTime(row.updated_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="240" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="goEdit(row)">编辑</el-button>
            <el-button link type="warning" @click="goVersions(row)">版本</el-button>
            <el-popconfirm
              title="确定删除此策略？"
              confirm-button-text="删除"
              cancel-button-text="取消"
              @confirm="doDelete(row)"
            >
              <template #reference>
                <el-button link type="danger">删除</el-button>
              </template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>

      <div v-if="!loading && filteredPolicies.length === 0" class="empty-state">
        <el-icon><DocumentDelete /></el-icon>
        <p>暂无策略数据</p>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search, Plus, Refresh } from '@element-plus/icons-vue'
import { listPolicies, deletePolicy, togglePolicy } from '@/api'
import dayjs from 'dayjs'

const router = useRouter()
const loading = ref(false)
const policies = ref([])
const searchKeyword = ref('')
const filterLevel = ref('')
const filterEffect = ref('')
const filterStatus = ref('')

const levelText = {
  global: '全局',
  tenant: '租户',
  project: '项目'
}

const filteredPolicies = computed(() => {
  let list = policies.value.map(p => ({ ...p, _status: p.status }))
  if (searchKeyword.value) {
    const kw = searchKeyword.value.toLowerCase()
    list = list.filter(p =>
      p.id.toLowerCase().includes(kw) ||
      (p.description && p.description.toLowerCase().includes(kw))
    )
  }
  if (filterLevel.value) list = list.filter(p => p.level === filterLevel.value)
  if (filterEffect.value) list = list.filter(p => p.effect === filterEffect.value)
  if (filterStatus.value) list = list.filter(p => p.status === filterStatus.value)
  return list.sort((a, b) => b.priority - a.priority || b.created_at - a.created_at)
})

const formatTime = (t) => dayjs(t).format('YYYY-MM-DD HH:mm:ss')

const loadPolicies = async () => {
  loading.value = true
  try {
    const data = await listPolicies()
    policies.value = data.policies || []
  } finally {
    loading.value = false
  }
}

const goCreate = () => router.push('/policies/new')
const goEdit = (row) => router.push(`/policies/${row.id}/edit`)
const goVersions = (row) => router.push(`/policies/${row.id}/versions`)

const toggleStatus = async (row) => {
  try {
    await togglePolicy(row.id, row._status)
    row.status = row._status
    ElMessage.success('状态已更新')
    loadPolicies()
  } catch (e) {
    row._status = row.status
  }
}

const doDelete = async (row) => {
  try {
    await deletePolicy(row.id)
    ElMessage.success('删除成功')
    loadPolicies()
  } catch (e) {}
}

onMounted(loadPolicies)
</script>

<style scoped>
.mono {
  font-family: monospace;
  font-size: 12px;
}
</style>
