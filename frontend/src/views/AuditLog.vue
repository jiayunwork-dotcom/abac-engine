<template>
  <div class="page-container">
    <div class="page-header">
      <h1 class="page-title">审计日志查询</h1>
      <div>
        <el-button @click="doExport" :disabled="!logs.length">
          <el-icon><Download /></el-icon>
          导出 CSV
        </el-button>
        <el-button type="primary" @click="queryLogs">
          <el-icon><Search /></el-icon>
          查询
        </el-button>
      </div>
    </div>

    <div class="card">
      <div class="filter-bar">
        <el-date-picker
          v-model="dateRange"
          type="datetimerange"
          range-separator="至"
          start-placeholder="开始时间"
          end-placeholder="结束时间"
          value-format="YYYY-MM-DDTHH:mm:ss"
          style="width: 400px;"
        />
        <el-select v-model="decision" placeholder="决策结果" clearable style="width: 140px;">
          <el-option label="允许" value="permit" />
          <el-option label="拒绝" value="deny" />
          <el-option label="不适用" value="not-applicable" />
        </el-select>
      </div>

      <el-row :gutter="16" style="margin-bottom: 16px;">
        <el-col :span="6">
          <div class="metric-card" style="background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);">
            <div class="metric-value">{{ total }}</div>
            <div class="metric-label">总记录数</div>
          </div>
        </el-col>
        <el-col :span="6">
          <div class="metric-card" style="background: linear-gradient(135deg, #11998e 0%, #38ef7d 100%);">
            <div class="metric-value">{{ stats.permit }}</div>
            <div class="metric-label">允许</div>
          </div>
        </el-col>
        <el-col :span="6">
          <div class="metric-card" style="background: linear-gradient(135deg, #eb3349 0%, #f45c43 100%);">
            <div class="metric-value">{{ stats.deny }}</div>
            <div class="metric-label">拒绝</div>
          </div>
        </el-col>
        <el-col :span="6">
          <div class="metric-card" style="background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%);">
            <div class="metric-value">{{ stats.avgTime }}µs</div>
            <div class="metric-label">平均耗时</div>
          </div>
        </el-col>
      </el-row>

      <el-table :data="logs" stripe v-loading="loading" style="width: 100%">
        <el-table-column label="时间" width="170">
          <template #default="{ row }">{{ formatTime(row.timestamp) }}</template>
        </el-table-column>
        <el-table-column prop="request_id" label="请求ID" width="180">
          <template #default="{ row }"><span class="mono">{{ row.request_id }}</span></template>
        </el-table-column>
        <el-table-column prop="project_id" label="项目" width="120" />
        <el-table-column prop="action" label="动作" width="100">
          <template #default="{ row }">
            <el-tag size="small" type="info">{{ row.action }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="decision" label="决策" width="90">
          <template #default="{ row }">
            <el-tag size="small" :class="row.decision === 'permit' ? 'tag-permit' : (row.decision === 'deny' ? 'tag-deny' : 'tag-disabled')">
              {{ decisionText(row.decision) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="命中策略" min-width="200" show-overflow-tooltip>
          <template #default="{ row }">
            <el-tag
              v-for="p in row.matched_policies"
              :key="p"
              size="small"
              class="tag-project"
              style="margin: 2px;"
            >{{ p }}</el-tag>
            <span v-if="!row.matched_policies?.length" style="color:#9ca3af;">-</span>
          </template>
        </el-table-column>
        <el-table-column prop="duration_us" label="耗时(µs)" width="100" sortable />
        <el-table-column label="详情" width="80">
          <template #default="{ row }">
            <el-button link type="primary" @click="showDetail(row)">查看</el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-pagination
        style="margin-top:16px;justify-content:flex-end;"
        v-model:current-page="page"
        v-model:page-size="pageSize"
        :page-sizes="[20, 50, 100, 200]"
        :total="total"
        layout="total, sizes, prev, pager, next, jumper"
        @size-change="queryLogs"
        @current-change="queryLogs"
      />
    </div>

    <el-dialog v-model="detailVisible" title="审计详情" width="700px">
      <div v-if="currentLog">
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item label="时间">{{ formatTime(currentLog.timestamp) }}</el-descriptions-item>
          <el-descriptions-item label="请求ID">{{ currentLog.request_id }}</el-descriptions-item>
          <el-descriptions-item label="租户ID">{{ currentLog.tenant_id }}</el-descriptions-item>
          <el-descriptions-item label="项目ID">{{ currentLog.project_id || '-' }}</el-descriptions-item>
          <el-descriptions-item label="动作">{{ currentLog.action }}</el-descriptions-item>
          <el-descriptions-item label="决策结果">
            <el-tag :class="currentLog.decision === 'permit' ? 'tag-permit' : 'tag-deny'" size="small">
              {{ decisionText(currentLog.decision) }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="耗时">{{ currentLog.duration_us }} µs</el-descriptions-item>
          <el-descriptions-item label="命中策略">
            {{ currentLog.matched_policies?.join(', ') || '-' }}
          </el-descriptions-item>
          <el-descriptions-item label="主体" :span="2">
            <pre class="json-viewer-sm">{{ currentLog.subject_summary }}</pre>
          </el-descriptions-item>
          <el-descriptions-item label="资源" :span="2">
            <pre class="json-viewer-sm">{{ currentLog.resource_summary }}</pre>
          </el-descriptions-item>
          <el-descriptions-item label="环境" :span="2">
            <pre class="json-viewer-sm">{{ currentLog.env_summary || '{}' }}</pre>
          </el-descriptions-item>
        </el-descriptions>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { Download, Search } from '@element-plus/icons-vue'
import { queryAudit, exportAudit } from '@/api'
import dayjs from 'dayjs'

const loading = ref(false)
const logs = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(50)
const dateRange = ref([
  dayjs().subtract(7, 'day').format('YYYY-MM-DDTHH:mm:ss'),
  dayjs().format('YYYY-MM-DDTHH:mm:ss')
])
const decision = ref('')
const detailVisible = ref(false)
const currentLog = ref(null)

const stats = computed(() => {
  const s = { permit: 0, deny: 0, other: 0, avgTime: 0 }
  let totalTime = 0
  logs.value.forEach(l => {
    if (l.decision === 'permit') s.permit++
    else if (l.decision === 'deny') s.deny++
    else s.other++
    totalTime += l.duration_us || 0
  })
  s.avgTime = logs.value.length ? Math.round(totalTime / logs.value.length) : 0
  return s
})

const formatTime = (t) => dayjs(t).format('YYYY-MM-DD HH:mm:ss')
const decisionText = (d) => ({ permit: '允许', deny: '拒绝', 'not-applicable': '不适用' }[d] || d)

const queryLogs = async () => {
  loading.value = true
  try {
    const params = {
      start: dateRange.value[0] + ':00',
      end: dateRange.value[1] + ':59',
      decision: decision.value,
      offset: (page.value - 1) * pageSize.value,
      limit: pageSize.value
    }
    const data = await queryAudit(params)
    logs.value = data.logs || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

const doExport = () => {
  const params = {
    start: dateRange.value[0] + ':00',
    end: dateRange.value[1] + ':59',
    decision: decision.value
  }
  exportAudit(params)
}

const showDetail = (row) => {
  currentLog.value = row
  detailVisible.value = true
}

onMounted(queryLogs)
</script>

<style scoped>
.mono { font-family: monospace; font-size: 12px; }
.json-viewer-sm {
  background: #f3f4f6;
  padding: 8px;
  border-radius: 4px;
  font-family: monospace;
  font-size: 11px;
  max-height: 120px;
  overflow: auto;
  margin: 0;
}
.metric-card { color: white; }
.metric-value { font-size: 24px; font-weight: 700; }
.metric-label { font-size: 12px; opacity: 0.9; margin-top: 4px; }
</style>
