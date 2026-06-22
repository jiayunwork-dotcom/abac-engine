<template>
  <div class="page-container">
    <div class="page-header">
      <h1 class="page-title">决策测试台 (What-if 模拟)</h1>
      <div>
        <el-button @click="loadSample">
          <el-icon><MagicStick /></el-icon>
          载入示例
        </el-button>
        <el-button type="primary" @click="runSimulate" :loading="running">
          <el-icon><VideoPlay /></el-icon>
          运行测试
        </el-button>
      </div>
    </div>

    <el-row :gutter="20">
      <el-col :span="14">
        <div class="card">
          <div class="section-title">构造访问请求</div>
          <el-form label-width="100px" size="default">
            <el-form-item label="项目ID">
              <el-input v-model="req.project_id" placeholder="可选，用于项目级策略匹配" />
            </el-form-item>

            <el-tabs v-model="activeTab" type="card" class="attr-tabs">
              <el-tab-pane label="主体 Subject" name="subject">
                <AttrEditor v-model="req.subject" :attrs="attrList?.subject || []" />
              </el-tab-pane>
              <el-tab-pane label="资源 Resource" name="resource">
                <AttrEditor v-model="req.resource" :attrs="attrList?.resource || []" />
              </el-tab-pane>
              <el-tab-pane label="动作 Action" name="action">
                <el-form-item label="动作名称" style="margin-top:12px;">
                  <el-select
                    v-model="req.action"
                    filterable
                    allow-create
                    default-first-option
                    placeholder="如 read, write, delete, approve, export"
                    style="width: 100%;"
                  >
                    <el-option v-for="a in actionList" :key="a" :label="a" :value="a" />
                  </el-select>
                </el-form-item>
              </el-tab-pane>
              <el-tab-pane label="环境 Environment" name="environment">
                <AttrEditor v-model="req.environment" :attrs="attrList?.environment || []" />
              </el-tab-pane>
            </el-tabs>
          </el-form>
        </div>

        <div class="card" style="margin-top:20px;">
          <div class="section-title">请求 JSON 预览</div>
          <pre class="json-viewer">{{ formatJSON(req) }}</pre>
        </div>
      </el-col>

      <el-col :span="10">
        <div class="card">
          <div class="section-title">决策结果</div>
          <div v-if="!result" class="empty-state">
            <el-icon><Cpu /></el-icon>
            <p>点击"运行测试"查看决策结果</p>
          </div>
          <div v-else>
            <el-row :gutter="12" style="margin-bottom: 16px;">
              <el-col :span="8">
                <div class="metric-card" :class="result.result.effect">
                  <div class="metric-value">{{ effectText }}</div>
                  <div class="metric-label">最终决策</div>
                </div>
              </el-col>
              <el-col :span="8">
                <div class="metric-card" style="background: linear-gradient(135deg, #4facfe 0%, #00f2fe 100%);">
                  <div class="metric-value">{{ result.result.decision_time_us }}µs</div>
                  <div class="metric-label">决策耗时</div>
                </div>
              </el-col>
              <el-col :span="8">
                <div class="metric-card" style="background: linear-gradient(135deg, #fa709a 0%, #fee140 100%);">
                  <div class="metric-value">{{ result.result.matched_policies?.length || 0 }}</div>
                  <div class="metric-label">命中策略数</div>
                </div>
              </el-col>
            </el-row>

            <el-alert
              :title="result.result.reason || '无'"
              type="info"
              :closable="false"
              style="margin-bottom: 16px;"
            />

            <div v-if="matchedPolicyNames.length" style="margin-bottom: 16px;">
              <div style="font-weight:600;margin-bottom:8px;">命中策略：</div>
              <div>
                <el-tag
                  v-for="p in matchedPolicyNames"
                  :key="p"
                  style="margin: 2px;"
                  class="tag-project"
                  size="small"
                >
                  {{ p }}
                </el-tag>
              </div>
            </div>

            <div class="section-title" style="margin-top:0;">决策过程 Trace</div>
            <div style="max-height: 400px; overflow: auto;">
              <div
                v-for="(t, i) in result.trace?.evaluated || []"
                :key="i"
                class="trace-item"
                :class="{ matched: t.matched, deny: t.matched && t.effect === 'deny' }"
              >
                <div class="flex-between">
                  <span>
                    <span class="mono" style="color:#6b7280;">[{{ t.level }}]</span>
                    <strong>{{ t.policy_id }}</strong>
                    <el-tag size="small" style="margin-left:6px;" :class="t.effect === 'permit' ? 'tag-permit' : 'tag-deny'">
                      {{ t.effect }}
                    </el-tag>
                    <span style="color:#9ca3af;margin-left:6px;">优先级:{{ t.priority }}</span>
                  </span>
                  <el-tag size="small" :type="t.matched ? 'success' : 'info'">
                    {{ t.matched ? '✓ 命中' : '✗ 未命中' }}
                  </el-tag>
                </div>
              </div>
            </div>
          </div>
        </div>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { MagicStick, VideoPlay, Cpu } from '@element-plus/icons-vue'
import { simulate, getAttributes } from '@/api'
import AttrEditor from '@/components/AttrEditor.vue'

const activeTab = ref('subject')
const running = ref(false)
const result = ref(null)
const attrList = ref(null)

const actionList = ['read','write','delete','approve','reject','export','import','create','update','list','manage','share','download','upload']

const req = reactive({
  project_id: '',
  subject: {},
  resource: {},
  action: 'read',
  environment: {}
})

const effectText = computed(() => {
  const e = result.value?.result?.effect
  return { permit: '✅ 允许', deny: '❌ 拒绝', 'not-applicable': '⚠ 不适用' }[e] || e
})

const matchedPolicyNames = computed(() => result.value?.result?.matched_policies || [])

const formatJSON = (obj) => JSON.stringify(obj, null, 2)

const loadSample = () => {
  Object.assign(req, {
    project_id: 'proj-finance',
    subject: {
      user_id: 'u1001',
      username: 'zhang.san',
      roles: ['finance_manager'],
      department: 'finance',
      level: 6,
      region: 'cn-north'
    },
    resource: {
      id: 'doc-2024-001',
      type: 'document',
      name: '2024财务预算.pdf',
      owner_id: 'u9999',
      owner_dept: 'finance',
      sensitivity_level: 'confidential',
      project_id: 'proj-finance',
      created_at: '2024-01-15T10:00:00+08:00'
    },
    action: 'export',
    environment: {
      timestamp: new Date().toISOString(),
      client_ip: '10.0.1.50',
      device_type: 'desktop',
      is_mfa_authenticated: true,
      country: 'CN'
    }
  })
  ElMessage.success('示例数据已载入')
}

const runSimulate = async () => {
  if (!req.action) {
    ElMessage.warning('请选择动作')
    return
  }
  running.value = true
  try {
    result.value = await simulate({ ...req })
    ElMessage.success('测试完成')
  } finally {
    running.value = false
  }
}

onMounted(async () => {
  try {
    attrList.value = await getAttributes()
  } catch (e) {}
})
</script>

<style scoped>
.attr-tabs { margin-top: 8px; }
.metric-card.permit { background: linear-gradient(135deg, #11998e 0%, #38ef7d 100%); }
.metric-card.deny { background: linear-gradient(135deg, #eb3349 0%, #f45c43 100%); }
.metric-card.not-applicable { background: linear-gradient(135deg, #8e9eab 0%, #eef2f3 100%); }
.metric-value { color: #fff; }
.metric-label { color: rgba(255,255,255,0.9); }
.flex-between { display:flex;justify-content:space-between;align-items:center; }
.mono { font-family: monospace; }
</style>
