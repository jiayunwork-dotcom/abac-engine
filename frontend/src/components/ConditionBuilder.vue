<template>
  <div class="condition-builder">
    <div v-if="!value" class="empty-group">
      <el-button type="primary" plain size="small" @click="initGroup">
        <el-icon><Plus /></el-icon>
        添加条件组
      </el-button>
    </div>

    <div v-else>
      <div class="group-header">
        <el-radio-group v-model="value.logic" size="small">
          <el-radio-button value="AND">全部满足 (AND)</el-radio-button>
          <el-radio-button value="OR">任一满足 (OR)</el-radio-button>
        </el-radio-group>
        <div>
          <el-button size="small" @click="addCondition">
            <el-icon><Plus /></el-icon>条件
          </el-button>
          <el-button size="small" type="success" plain @click="addSubGroup">
            <el-icon><FolderAdd /></el-icon>嵌套组
          </el-button>
          <el-button size="small" type="danger" plain @click="removeGroup" v-if="isRoot">
            <el-icon><Delete /></el-icon>
          </el-button>
        </div>
      </div>

      <div class="conditions-list">
        <div
          v-for="(cond, idx) in value.conditions"
          :key="'c'+idx"
          class="cond-row"
        >
          <el-select
            v-model="cond.attribute"
            placeholder="属性"
            filterable
            size="small"
            style="width: 180px;"
          >
            <el-option
              v-for="attr in attributes"
              :key="attr"
              :label="attr"
              :value="attr"
            />
          </el-select>
          <el-select
            v-model="cond.operator"
            placeholder="操作符"
            size="small"
            style="width: 150px;"
          >
            <el-option v-for="op in operators" :key="op" :label="opLabel[op] || op" :value="op" />
          </el-select>
          <div class="value-input-wrap">
            <el-input
              v-if="isSimpleValueOp(cond.operator)"
              v-model="cond.value"
              placeholder="值"
              size="small"
              clearable
              @change="$emit('update:modelValue', value)"
            />
            <el-select
              v-else-if="cond.operator === 'in' || cond.operator === 'not_in' || cond.operator === 'intersects'"
              v-model="cond.value"
              multiple
              filterable
              allow-create
              default-first-option
              size="small"
              placeholder="多选，输入回车添加"
              style="width: 100%;"
            />
            <el-switch
              v-else-if="cond.operator === 'exists'"
              v-model="cond.value"
            />
            <div v-else-if="cond.operator === 'time_range'" class="time-range">
              <el-time-picker
                v-model="cond.value.start"
                format="HH:mm"
                value-format="HH:mm"
                placeholder="开始"
                size="small"
                style="width: 100px;"
              />
              <span>~</span>
              <el-time-picker
                v-model="cond.value.end"
                format="HH:mm"
                value-format="HH:mm"
                placeholder="结束"
                size="small"
                style="width: 100px;"
              />
            </div>
            <el-select
              v-else-if="cond.operator === 'weekday_range'"
              v-model="cond.value"
              multiple
              size="small"
              placeholder="选择星期"
              style="width: 100%;"
            >
              <el-option v-for="(w, i) in weekdays" :key="i" :label="w" :value="i" />
            </el-select>
            <div v-else style="flex:1;">
              <el-input
                v-model="cond.value_str"
                placeholder="JSON 值，如 {start:'09:00',end:'18:00'}"
                size="small"
                @change="parseJSONValue(cond)"
              />
            </div>
          </div>
          <el-button size="small" type="danger" text @click="removeCondition(idx)">
            <el-icon><Delete /></el-icon>
          </el-button>
        </div>
      </div>

      <div v-if="value.groups && value.groups.length" class="subgroups">
        <div v-for="(g, idx) in value.groups" :key="'g'+idx" class="subgroup">
          <ConditionBuilder
            :modelValue="g"
            @update:modelValue="val => updateSubGroup(idx, val)"
            :attributes="attributes"
            :isRoot="false"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { watch } from 'vue'
import { useAppStore } from '@/stores/app'

const props = defineProps({
  modelValue: { type: Object, default: null },
  attributes: { type: Array, default: () => [] },
  isRoot: { type: Boolean, default: true }
})
const emit = defineEmits(['update:modelValue'])
const store = useAppStore()

const value = props.modelValue
const v = value

const operatorList = [
  'equals','not_equals','contains','not_contains','regex_match',
  'gt','gte','lt','lte','in','not_in',
  'ip_in_cidr','time_range','weekday_range','intersects','exists'
]
const operators = store.attributes?.operators || operatorList

const opLabel = {
  equals: '=', not_equals: '≠', contains: '包含', not_contains: '不包含',
  regex_match: '正则匹配', gt: '>', gte: '≥', lt: '<', lte: '≤',
  in: '属于', not_in: '不属于', ip_in_cidr: 'IP在网段',
  time_range: '时间范围', weekday_range: '星期范围',
  intersects: '集合相交', exists: '属性存在'
}

const weekdays = ['周日','周一','周二','周三','周四','周五','周六']

watch(() => props.modelValue, (nv) => {
  if (nv && nv.conditions) {
    nv.conditions.forEach(c => {
      if (typeof c.value === 'object' && c.value !== null && !(c.value instanceof Array)) {
        c.value_str = JSON.stringify(c.value)
      }
    })
  }
}, { immediate: true, deep: true })

const initGroup = () => {
  const g = { logic: 'AND', conditions: [{ attribute: '', operator: 'equals', value: '' }], groups: [] }
  emit('update:modelValue', g)
}

const addCondition = () => {
  if (!v.conditions) v.conditions = []
  v.conditions.push({ attribute: '', operator: 'equals', value: '' })
  emit('update:modelValue', v)
}

const removeCondition = (idx) => {
  v.conditions.splice(idx, 1)
  emit('update:modelValue', v)
}

const addSubGroup = () => {
  if (!v.groups) v.groups = []
  v.groups.push({ logic: 'AND', conditions: [], groups: [] })
  emit('update:modelValue', v)
}

const removeGroup = () => {
  emit('update:modelValue', null)
}

const updateSubGroup = (idx, g) => {
  if (!g) {
    v.groups.splice(idx, 1)
  } else {
    v.groups[idx] = g
  }
  emit('update:modelValue', v)
}

const isSimpleValueOp = (op) => ['equals','not_equals','contains','not_contains','regex_match',
  'gt','gte','lt','lte','ip_in_cidr'].includes(op)

const parseJSONValue = (cond) => {
  try {
    if (cond.value_str && cond.value_str.trim().startsWith('{')) {
      cond.value = JSON.parse(cond.value_str.replace(/'/g, '"'))
    }
  } catch (e) {}
}
</script>

<style scoped>
.empty-group {
  text-align: center;
  padding: 30px;
  color: #9ca3af;
}
.group-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
  padding: 8px;
  background: #fff;
  border-radius: 4px;
  border: 1px solid #e5e7eb;
}
.cond-row {
  display: flex;
  gap: 8px;
  align-items: center;
  padding: 8px;
  background: #fff;
  border-radius: 4px;
  margin-bottom: 6px;
  border: 1px solid #f3f4f6;
}
.value-input-wrap {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 6px;
}
.time-range {
  display: flex;
  align-items: center;
  gap: 4px;
  flex: 1;
}
.subgroups {
  margin-left: 20px;
  padding-left: 12px;
  border-left: 2px dashed #d1d5db;
}
.subgroup {
  margin: 8px 0;
}
</style>
