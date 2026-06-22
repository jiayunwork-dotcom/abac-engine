<template>
  <div class="page-container">
    <div class="page-header">
      <h1 class="page-title">
        <el-button text @click="goBack">
          <el-icon><ArrowLeft /></el-icon>
        </el-button>
        {{ isEdit ? '编辑策略' : '新建策略' }}
      </h1>
      <div>
        <el-button @click="doValidate">
          <el-icon><CircleCheck /></el-icon>
          校验
        </el-button>
        <el-button type="primary" @click="doSave" :loading="saving">
          <el-icon><Check /></el-icon>
          保存
        </el-button>
      </div>
    </div>

    <el-row :gutter="20">
      <el-col :span="14">
        <div class="card">
          <div class="section-title">策略配置 (YAML)</div>
          <div class="code-editor" style="min-height: 500px;">
            <textarea
              v-model="yamlContent"
              spellcheck="false"
              style="width:100%;min-height:500px;padding:16px;border:none;outline:none;resize:vertical;font-family:Monaco,Menlo,monospace;font-size:13px;"
            ></textarea>
          </div>
          <div v-if="validationErrors.length" class="mt-16">
            <el-alert type="error" :closable="false">
              <div slot="default">
                <div v-for="(e, i) in validationErrors" :key="i" class="error-line">
                  • {{ e }}
                </div>
              </div>
            </el-alert>
          </div>
        </div>
      </el-col>

      <el-col :span="10">
        <div class="card" style="margin-bottom: 20px;">
          <div class="section-title">基础信息</div>
          <el-form label-width="100px" size="default">
            <el-form-item label="策略ID">
              <el-input v-model="form.id" :disabled="isEdit" placeholder="留空自动生成" />
            </el-form-item>
            <el-form-item label="描述">
              <el-input v-model="form.description" placeholder="策略描述" />
            </el-form-item>
            <el-form-item label="层级">
              <el-select v-model="form.level" style="width: 100%;">
                <el-option label="项目级" value="project" />
                <el-option label="租户级" value="tenant" />
                <el-option label="全局级" value="global" disabled />
              </el-select>
            </el-form-item>
            <el-form-item v-if="form.level === 'project'" label="项目ID">
              <el-input v-model="form.project_id" />
            </el-form-item>
            <el-form-item label="效果">
              <el-radio-group v-model="form.effect">
                <el-radio value="permit">允许 (Permit)</el-radio>
                <el-radio value="deny">拒绝 (Deny)</el-radio>
              </el-radio-group>
            </el-form-item>
            <el-form-item label="优先级">
              <el-input-number v-model="form.priority" :min="0" :max="10000" style="width:100%" />
            </el-form-item>
            <el-form-item label="资源类型">
              <el-select
                v-model="form.resource_types"
                multiple
                filterable
                allow-create
                default-first-option
                style="width: 100%;"
                placeholder="逗号分隔，如 document, user"
              >
                <el-option label="document" value="document" />
                <el-option label="user" value="user" />
                <el-option label="order" value="order" />
                <el-option label="report" value="report" />
                <el-option label="api" value="api" />
              </el-select>
            </el-form-item>
            <el-form-item label="操作">
              <el-select
                v-model="form.actions"
                multiple
                filterable
                allow-create
                default-first-option
                style="width: 100%;"
                placeholder="逗号分隔，如 read, write"
              >
                <el-option label="read" value="read" />
                <el-option label="write" value="write" />
                <el-option label="delete" value="delete" />
                <el-option label="approve" value="approve" />
                <el-option label="export" value="export" />
              </el-select>
            </el-form-item>
            <el-form-item label="强制拒绝">
              <el-switch v-model="form.force_deny" />
              <span class="tip-text">（全局策略中deny不可被下级覆盖）</span>
            </el-form-item>
            <el-form-item label="变更说明">
              <el-input v-model="changeNote" placeholder="本次变更的说明（可选）" type="textarea" :rows="2" />
            </el-form-item>
          </el-form>
        </div>

        <div class="card">
          <div class="section-title">
            可视化条件构建器
            <el-button size="small" type="primary" plain style="float:right" @click="generateYAML">
              生成 YAML
            </el-button>
          </div>

          <el-tabs v-model="activeDim" type="card" class="cond-tabs">
            <el-tab-pane label="主体 Subject" name="subject">
              <ConditionBuilder
                v-model="form.target.subject"
                :attributes="attrs?.subject || []"
                ref="subBuilder"
              />
            </el-tab-pane>
            <el-tab-pane label="资源 Resource" name="resource">
              <ConditionBuilder
                v-model="form.target.resource"
                :attributes="attrs?.resource || []"
              />
            </el-tab-pane>
            <el-tab-pane label="动作 Action" name="action">
              <ConditionBuilder
                v-model="form.target.action"
                :attributes="attrs?.action || []"
              />
            </el-tab-pane>
            <el-tab-pane label="环境 Environment" name="environment">
              <ConditionBuilder
                v-model="form.target.environment"
                :attributes="attrs?.environment || []"
              />
            </el-tab-pane>
          </el-tabs>
        </div>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ArrowLeft, CircleCheck, Check } from '@element-plus/icons-vue'
import { getPolicy, createPolicy, updatePolicy, validatePolicy, getAttributes } from '@/api'
import ConditionBuilder from '@/components/ConditionBuilder.vue'
import YAML from 'yaml'

const route = useRoute()
const router = useRouter()

const isEdit = computed(() => !!route.params.id)
const policyId = computed(() => route.params.id)

const activeDim = ref('subject')
const attrs = ref(null)
const yamlContent = ref('')
const validationErrors = ref([])
const saving = ref(false)
const changeNote = ref('')

const defaultTarget = {
  subject: null,
  resource: null,
  action: null,
  environment: null
}

const form = reactive({
  id: '',
  description: '',
  level: 'tenant',
  project_id: '',
  effect: 'permit',
  priority: 100,
  status: 'enabled',
  version: 1,
  force_deny: false,
  resource_types: [],
  actions: [],
  target: { ...defaultTarget }
})

const goBack = () => router.push('/policies')

const YAMLTemplate = `id: pol-demo-example
description: 示例策略 - 文档部门可读
level: tenant
target:
  resource:
    conditions:
      - attribute: type
        operator: equals
        value: document
  subject:
    conditions:
      - attribute: department
        operator: equals
        value: docs
  action:
    conditions:
      - attribute: name
        operator: in
        value:
          - read
          - export
effect: permit
priority: 100
status: enabled
resource_types:
  - document
actions:
  - read
  - export
`

const parseYAML = () => {
  try {
    const obj = YAML.parse(yamlContent.value) || {}
    Object.keys(form).forEach(k => {
      if (k in obj) {
        if (k === 'target') {
          form.target = { ...defaultTarget, ...obj.target }
        } else {
          form[k] = obj[k]
        }
      }
    })
    validationErrors.value = []
  } catch (e) {
    validationErrors.value = ['YAML 解析错误: ' + e.message]
  }
}

watch(yamlContent, () => {
  try { YAML.parse(yamlContent.value) } catch (e) {}
}, { deep: true })

const generateYAML = () => {
  const policyObj = { ...form }
  policyObj.target = {}
  ;['subject', 'resource', 'action', 'environment'].forEach(dim => {
    if (form.target[dim] && (form.target[dim].conditions?.length || form.target[dim].groups?.length)) {
      policyObj.target[dim] = form.target[dim]
    }
  })
  yamlContent.value = YAML.stringify(policyObj, { lineWidth: 0, singleQuote: false })
  ElMessage.success('已生成 YAML，请检查左侧')
}

const doValidate = async () => {
  try {
    const res = await validatePolicy(yamlContent.value)
    if (res.valid) {
      validationErrors.value = []
      ElMessage.success('校验通过')
    } else {
      validationErrors.value = res.errors || []
      ElMessage.error('校验不通过，请检查错误')
    }
  } catch (e) {}
}

const doSave = async () => {
  saving.value = true
  try {
    const checkRes = await validatePolicy(yamlContent.value)
    if (!checkRes.valid) {
      validationErrors.value = checkRes.errors || []
      ElMessage.error('校验不通过，无法保存')
      return
    }
    const payload = { yaml: yamlContent.value, change_note: changeNote.value, created_by: 'web-ui' }
    if (isEdit.value) {
      await updatePolicy(policyId.value, payload)
      ElMessage.success('策略已更新')
    } else {
      await createPolicy(payload)
      ElMessage.success('策略已创建')
    }
    setTimeout(() => router.push('/policies'), 500)
  } finally {
    saving.value = false
  }
}

const loadPolicy = async () => {
  if (!isEdit.value) {
    yamlContent.value = YAMLTemplate
    parseYAML()
    return
  }
  try {
    const data = await getPolicy(policyId.value)
    yamlContent.value = data.yaml
    parseYAML()
  } catch (e) {}
}

onMounted(async () => {
  try {
    attrs.value = await getAttributes()
  } catch (e) {}
  await loadPolicy()
})
</script>

<style scoped>
.mt-16 { margin-top: 16px; }
.tip-text { font-size: 12px; color: #9ca3af; margin-left: 8px; }
.cond-tabs { margin-top: 8px; }
.error-line { font-size: 13px; line-height: 1.8; }
</style>
