<template>
  <div class="attr-editor">
    <div v-for="(attr, idx) in editorRows" :key="attr._id" class="attr-row">
      <el-select
        v-model="attr.name"
        placeholder="属性名"
        filterable
        allow-create
        default-first-option
        size="small"
        style="width: 180px;"
        @change="updateValue"
      >
        <el-option v-for="a in attributes" :key="a" :label="a" :value="a" />
      </el-select>
      <el-select
        v-model="attr.type"
        placeholder="类型"
        size="small"
        style="width: 100px;"
        @change="updateValue"
      >
        <el-option label="字符串" value="string" />
        <el-option label="数字" value="number" />
        <el-option label="布尔" value="boolean" />
        <el-option label="数组" value="array" />
      </el-select>
      <el-input
        v-if="attr.type === 'string'"
        v-model="attr.rawValue"
        placeholder="值"
        size="small"
        @change="updateValue"
      />
      <el-input-number
        v-else-if="attr.type === 'number'"
        v-model="attr.numValue"
        size="small"
        style="width: 100%;"
        @change="updateValue"
      />
      <el-switch
        v-else-if="attr.type === 'boolean'"
        v-model="attr.boolValue"
        @change="updateValue"
      />
      <el-select
        v-else-if="attr.type === 'array'"
        v-model="attr.arrValue"
        multiple
        filterable
        allow-create
        default-first-option
        size="small"
        placeholder="多项，回车添加"
        style="width: 100%;"
        @change="updateValue"
      />
      <el-button size="small" type="danger" text @click="removeRow(idx)">
        <el-icon><Delete /></el-icon>
      </el-button>
    </div>
    <el-button size="small" type="primary" plain @click="addRow" style="margin-top: 8px;">
      <el-icon><Plus /></el-icon>
      添加属性
    </el-button>
  </div>
</template>

<script setup>
import { ref, watch, onMounted } from 'vue'

const props = defineProps({
  modelValue: { type: Object, default: () => ({}) },
  attributes: { type: Array, default: () => [] }
})
const emit = defineEmits(['update:modelValue'])

let idCounter = 0
const nextId = () => ++idCounter

const rowFromKV = (k, v) => {
  const r = { _id: nextId(), name: k, type: 'string', rawValue: '', numValue: 0, boolValue: false, arrValue: [] }
  if (typeof v === 'number') { r.type = 'number'; r.numValue = v }
  else if (typeof v === 'boolean') { r.type = 'boolean'; r.boolValue = v }
  else if (Array.isArray(v)) { r.type = 'array'; r.arrValue = v.map(x => String(x)) }
  else { r.type = 'string'; r.rawValue = v == null ? '' : String(v) }
  return r
}

const editorRows = ref([])

const syncFromModel = () => {
  editorRows.value = Object.entries(props.modelValue || {})
    .filter(([k]) => k !== '_id')
    .map(([k, v]) => rowFromKV(k, v))
}

onMounted(syncFromModel)
watch(() => props.modelValue, syncFromModel, { deep: true })

const addRow = () => {
  editorRows.value.push({
    _id: nextId(), name: '', type: 'string', rawValue: '', numValue: 0, boolValue: false, arrValue: []
  })
}

const removeRow = (idx) => {
  editorRows.value.splice(idx, 1)
  updateValue()
}

const updateValue = () => {
  const out = {}
  editorRows.value.forEach(r => {
    if (!r.name) return
    let v
    if (r.type === 'number') v = r.numValue
    else if (r.type === 'boolean') v = r.boolValue
    else if (r.type === 'array') v = r.arrValue
    else v = r.rawValue
    out[r.name] = v
  })
  emit('update:modelValue', out)
}
</script>

<style scoped>
.attr-row {
  display: flex;
  gap: 8px;
  align-items: center;
  padding: 6px 0;
}
.attr-row > *:nth-child(3),
.attr-row > *:nth-child(4),
.attr-row > *:nth-child(5) {
  flex: 1;
}
</style>
