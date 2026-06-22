<template>
  <div class="page-container">
    <div class="page-header">
      <h1 class="page-title">系统设置</h1>
    </div>

    <el-row :gutter="20">
      <el-col :span="12">
        <div class="card">
          <div class="section-title">租户访问凭证</div>
          <el-form label-width="120px">
            <el-form-item label="API Key">
              <el-input
                v-model="store.apiKey"
                type="textarea"
                :rows="2"
                placeholder="租户 API Key，用于访问 ABAC API"
              />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="saveApiKey">保存 API Key</el-button>
            </el-form-item>
          </el-form>
          <el-alert
            title="默认 Demo 租户密钥：sk-demo-tenant-key-12345"
            type="info"
            :closable="false"
          />
        </div>

        <div class="card" style="margin-top: 20px;">
          <div class="section-title">平台管理员凭证</div>
          <el-form label-width="120px">
            <el-form-item label="管理员 Token">
              <el-input
                v-model="store.adminToken"
                placeholder="用于管理租户的管理员权限"
                type="password"
                show-password
              />
            </el-form-item>
            <el-form-item>
              <el-button type="warning" @click="saveAdminToken">保存管理员 Token</el-button>
            </el-form-item>
          </el-form>
          <el-alert
            title="默认管理员 Token：admin-secret-token-change-me（请在生产环境修改）"
            type="warning"
            :closable="false"
          />
        </div>
      </el-col>

      <el-col :span="12">
        <div class="card">
          <div class="section-title">支持的属性定义</div>
          <el-collapse>
            <el-collapse-item title="主体属性 Subject" name="subject">
              <div v-for="attr in attrList?.subject || []" :key="attr" class="attr-item">
                <el-tag size="small" type="success">{{ attr }}</el-tag>
              </div>
            </el-collapse-item>
            <el-collapse-item title="资源属性 Resource" name="resource">
              <div v-for="attr in attrList?.resource || []" :key="attr" class="attr-item">
                <el-tag size="small" type="warning">{{ attr }}</el-tag>
              </div>
            </el-collapse-item>
            <el-collapse-item title="动作属性 Action" name="action">
              <div v-for="attr in attrList?.action || []" :key="attr" class="attr-item">
                <el-tag size="small" type="primary">{{ attr }}</el-tag>
              </div>
            </el-collapse-item>
            <el-collapse-item title="环境属性 Environment" name="environment">
              <div v-for="attr in attrList?.environment || []" :key="attr" class="attr-item">
                <el-tag size="small" type="info">{{ attr }}</el-tag>
              </div>
            </el-collapse-item>
            <el-collapse-item title="支持的操作符" name="operators">
              <el-table :data="operatorList" size="small" stripe>
                <el-table-column prop="op" label="操作符" width="140" />
                <el-table-column prop="desc" label="说明" />
                <el-table-column prop="example" label="示例值" width="200" />
              </el-table>
            </el-collapse-item>
          </el-collapse>
        </div>

        <div class="card" style="margin-top: 20px;">
          <div class="section-title">组合算法说明</div>
          <el-table :data="algoList" size="small" stripe>
            <el-table-column prop="name" label="算法" width="160" />
            <el-table-column prop="desc" label="决策逻辑" />
          </el-table>
        </div>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { useAppStore } from '@/stores/app'
import { getAttributes } from '@/api'

const store = useAppStore()
const attrList = ref(null)

const operatorList = [
  { op: 'equals', desc: '等于', example: 'finance' },
  { op: 'not_equals', desc: '不等于', example: 'public' },
  { op: 'contains', desc: '包含（字符串/数组）', example: 'admin' },
  { op: 'not_contains', desc: '不包含', example: 'guest' },
  { op: 'regex_match', desc: '正则匹配', example: '^u\\d+$' },
  { op: 'gt/gte/lt/lte', desc: '数值大小比较', example: '100' },
  { op: 'in', desc: '属于集合', example: '["read","write"]' },
  { op: 'not_in', desc: '不属于集合', example: '["delete"]' },
  { op: 'ip_in_cidr', desc: 'IP 在网段内', example: '10.0.0.0/8' },
  { op: 'time_range', desc: '时间范围内', example: '{start:"09:00",end:"18:00"}' },
  { op: 'weekday_range', desc: '星期范围内', example: '[1,2,3,4,5]' },
  { op: 'intersects', desc: '两个集合有交集', example: '["fin","hr"]' },
  { op: 'exists', desc: '属性存在/不存在', example: 'true/false' }
]

const algoList = [
  { name: 'Deny-Override 拒绝优先', desc: '只要有任一条匹配的 deny 策略，最终即 deny；否则全部 permit 时 permit' },
  { name: 'Permit-Override 允许优先', desc: '只要有任一条匹配的 permit 策略，最终即 permit；否则全部 deny 时 deny' },
  { name: 'Priority-First 优先级优先', desc: '取所有匹配策略中优先级最高的那条的效果' }
]

const saveApiKey = () => {
  store.setApiKey(store.apiKey)
  ElMessage.success('API Key 已保存')
}

const saveAdminToken = () => {
  store.setAdminToken(store.adminToken)
  ElMessage.success('管理员 Token 已保存')
}

onMounted(async () => {
  try {
    attrList.value = await getAttributes()
    if (!store.attributes) store.setAttributes(attrList.value)
  } catch (e) {}
})
</script>

<style scoped>
.attr-item {
  display: inline-block;
  margin: 4px 6px;
}
</style>
