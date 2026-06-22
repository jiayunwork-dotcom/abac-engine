<template>
  <div class="page-container">
    <div class="page-header">
      <h1 class="page-title">
        <el-button text @click="goBack">
          <el-icon><ArrowLeft /></el-icon>
        </el-button>
        版本历史 - {{ policyId }}
      </h1>
      <div>
        <el-button :disabled="!selectedA || !selectedB" @click="showDiff = true">
          <el-icon><Comparison /></el-icon>
          对比版本
        </el-button>
      </div>
    </div>

    <div class="card">
      <el-alert
        title="回滚操作会创建一个新版本，保留完整历史（不可逆）"
        type="info"
        :closable="false"
        style="margin-bottom: 16px;"
      />
      <div v-if="versions.length === 0" class="empty-state">
        <el-icon><Clock /></el-icon>
        <p>暂无版本记录</p>
      </div>
      <el-table :data="versions" stripe v-loading="loading">
        <el-table-column label="选择对比" width="180">
          <template #default="{ row }">
            <div style="display: flex; gap: 8px;">
              <el-radio
                v-model="selectedA"
                :value="row.version"
                :label="row.version"
                @change="sel => {}"
              >A</el-radio>
              <el-radio
                v-model="selectedB"
                :value="row.version"
                :label="row.version"
              >B</el-radio>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="版本" width="90">
          <template #default="{ row }">
            <el-tag size="small" type="primary">v{{ row.version }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="change_note" label="变更说明" min-width="200" show-overflow-tooltip />
        <el-table-column prop="created_by" label="操作人" width="120" />
        <el-table-column label="时间" width="170">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="160" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="viewVersion(row)">查看</el-button>
            <el-popconfirm
              :title="`确定回滚到 v${row.version}？`"
              confirm-button-text="回滚"
              @confirm="doRollback(row)"
            >
              <template #reference>
                <el-button link type="warning">回滚到此</el-button>
              </template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog v-model="viewDialog" title="版本详情" width="800px">
      <div v-if="currentVersion">
        <el-descriptions :column="2" border size="small" style="margin-bottom: 12px;">
          <el-descriptions-item label="版本">v{{ currentVersion.version }}</el-descriptions-item>
          <el-descriptions-item label="操作人">{{ currentVersion.created_by }}</el-descriptions-item>
          <el-descriptions-item label="时间">{{ formatTime(currentVersion.created_at) }}</el-descriptions-item>
          <el-descriptions-item label="说明">{{ currentVersion.change_note }}</el-descriptions-item>
        </el-descriptions>
        <div class="section-title">策略内容 (YAML/JSON)</div>
        <pre class="json-viewer" style="max-height: 400px;">{{ prettyContent(currentVersion.content) }}</pre>
      </div>
    </el-dialog>

    <el-dialog v-model="showDiff" title="版本对比" width="900px">
      <div v-if="versionA && versionB">
        <div style="margin-bottom: 12px; display: flex; gap: 20px;">
          <el-tag type="info">版本 A: v{{ versionA.version }}</el-tag>
          <el-tag type="primary">版本 B: v{{ versionB.version }}</el-tag>
        </div>
        <el-row :gutter="16">
          <el-col :span="12">
            <div class="section-title">v{{ versionA.version }}</div>
            <pre class="json-viewer" style="max-height: 500px;">{{ prettyContent(versionA.content) }}</pre>
          </el-col>
          <el-col :span="12">
            <div class="section-title">v{{ versionB.version }}</div>
            <pre class="json-viewer" style="max-height: 500px;">{{ prettyContent(versionB.content) }}</pre>
          </el-col>
        </el-row>
      </div>
      <div v-else>
        <el-empty description="请先选择两个版本进行对比" />
      </div>
    </el-dialog>

    <el-dialog v-model="rollbackDialog" title="回滚确认" width="450px">
      <el-form label-width="100px">
        <el-form-item label="目标版本">
          <el-tag type="warning">v{{ rollbackVersion }}</el-tag>
        </el-form-item>
        <el-form-item label="回滚说明">
          <el-input v-model="rollbackNote" type="textarea" :rows="3" placeholder="说明本次回滚的原因" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="rollbackDialog = false">取消</el-button>
        <el-button type="warning" @click="confirmRollback" :loading="rolling">确认回滚</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ArrowLeft, Comparison, Clock } from '@element-plus/icons-vue'
import { listVersions, rollbackPolicy } from '@/api'
import dayjs from 'dayjs'
import YAML from 'yaml'

const route = useRoute()
const router = useRouter()
const policyId = computed(() => route.params.id)

const loading = ref(false)
const versions = ref([])
const selectedA = ref(null)
const selectedB = ref(null)
const viewDialog = ref(false)
const currentVersion = ref(null)
const showDiff = ref(false)

const rollbackDialog = ref(false)
const rollbackVersion = ref(0)
const rollbackNote = ref('')
const rolling = ref(false)

const versionA = computed(() => versions.value.find(v => v.version === selectedA.value))
const versionB = computed(() => versions.value.find(v => v.version === selectedB.value))

const formatTime = (t) => dayjs(t).format('YYYY-MM-DD HH:mm:ss')
const goBack = () => router.push('/policies')

const prettyContent = (content) => {
  try {
    const obj = JSON.parse(content)
    return YAML.stringify(obj, { lineWidth: 0 })
  } catch {
    try {
      YAML.parse(content)
      return content
    } catch {
      return content
    }
  }
}

const loadVersions = async () => {
  loading.value = true
  try {
    const data = await listVersions(policyId.value)
    versions.value = data.versions || []
  } finally {
    loading.value = false
  }
}

const viewVersion = (row) => {
  currentVersion.value = row
  viewDialog.value = true
}

const doRollback = (row) => {
  rollbackVersion.value = row.version
  rollbackNote.value = ''
  rollbackDialog.value = true
}

const confirmRollback = async () => {
  if (!rollbackNote.value) {
    ElMessage.warning('请填写回滚说明')
    return
  }
  rolling.value = true
  try {
    await rollbackPolicy(policyId.value, {
      version: rollbackVersion.value,
      note: rollbackNote.value,
      created_by: 'web-ui'
    })
    ElMessage.success(`已回滚到 v${rollbackVersion.value}`)
    rollbackDialog.value = false
    setTimeout(() => {
      router.push('/policies')
    }, 800)
  } finally {
    rolling.value = false
  }
}

onMounted(loadVersions)
</script>
