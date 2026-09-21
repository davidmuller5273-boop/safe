<template>
  <div>
    <div class="page-head">
      <h2>彩种管理</h2>
      <el-button type="primary" @click="openDialog()">新增彩种</el-button>
    </div>
    <el-table v-loading="loading" :data="items" border>
      <el-table-column prop="name" label="彩种名称" min-width="180" />
      <el-table-column prop="symbol" label="Symbol" min-width="140" />
      <el-table-column prop="seconds_per_issue" label="多少秒一期" min-width="140"><template #default="scope">{{ scope.row.seconds_per_issue }} 秒</template></el-table-column>
      <el-table-column prop="issues_per_day" label="每天多少期" min-width="140"><template #default="scope">{{ scope.row.issues_per_day }} 期</template></el-table-column>
      <el-table-column prop="draws_all_day" label="是否全天开奖" min-width="140"><template #default="scope"><el-tag :type="scope.row.draws_all_day ? 'success' : 'info'">{{ scope.row.draws_all_day ? '是' : '否' }}</el-tag></template></el-table-column>
      <el-table-column label="操作" width="150" fixed="right"><template #default="scope"><el-button link type="primary" @click="openDialog(scope.row)">编辑</el-button><el-button link type="danger" @click="remove(scope.row)">删除</el-button></template></el-table-column>
      <template #empty>暂无彩种</template>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="form.id ? '编辑彩种' : '新增彩种'" width="520px" destroy-on-close @closed="resetForm">
      <el-form ref="formRef" :model="form" :rules="rules" label-width="120px">
        <el-form-item label="彩种名称" prop="name"><el-input v-model.trim="form.name" maxlength="100" show-word-limit placeholder="请输入彩种名称" /></el-form-item>
        <el-form-item label="Symbol" prop="symbol"><el-input v-model.trim="form.symbol" maxlength="64" show-word-limit placeholder="请输入彩种标识" /></el-form-item>
        <el-form-item label="多少秒一期" prop="seconds_per_issue"><el-input-number v-model="form.seconds_per_issue" :min="1" :max="86400" :step="1" controls-position="right" /><span class="unit">秒</span></el-form-item>
        <el-form-item label="每天多少期" prop="issues_per_day"><el-input-number v-model="form.issues_per_day" :min="1" :max="100000" :step="1" controls-position="right" /><span class="unit">期</span></el-form-item>
        <el-form-item label="是否全天开奖" prop="draws_all_day"><el-switch v-model="form.draws_all_day" inline-prompt active-text="是" inactive-text="否" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="dialogVisible = false">取消</el-button><el-button type="primary" :loading="submitting" @click="save">保存</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { createLotteryType, deleteLotteryType, getLotteryTypes, updateLotteryType } from '@/api'
import type { LotteryType } from '@/types'

const items = ref<LotteryType[]>([])
const loading = ref(false)
const dialogVisible = ref(false)
const submitting = ref(false)
const formRef = ref<FormInstance>()
const form = reactive({ id: 0, name: '', symbol: '', seconds_per_issue: 60, issues_per_day: 1, draws_all_day: false })
const rules: FormRules = {
  name: [{ required: true, message: '请输入彩种名称', trigger: 'blur' }],
  symbol: [{ required: true, message: '请输入彩种标识', trigger: 'blur' }],
  seconds_per_issue: [{ required: true, message: '请输入每期秒数', trigger: 'change' }],
  issues_per_day: [{ required: true, message: '请输入每天期数', trigger: 'change' }],
}

async function load() {
  loading.value = true
  try { items.value = (await getLotteryTypes()).data } finally { loading.value = false }
}

function openDialog(item?: LotteryType) {
  Object.assign(form, item ? { id: item.id, name: item.name, symbol: item.symbol, seconds_per_issue: item.seconds_per_issue, issues_per_day: item.issues_per_day, draws_all_day: item.draws_all_day } : { id: 0, name: '', symbol: '', seconds_per_issue: 60, issues_per_day: 1, draws_all_day: false })
  dialogVisible.value = true
}

function resetForm() { formRef.value?.resetFields() }

async function save() {
  if (!formRef.value || !await formRef.value.validate().catch(() => false)) return
  submitting.value = true
  const data = { name: form.name, symbol: form.symbol, seconds_per_issue: form.seconds_per_issue, issues_per_day: form.issues_per_day, draws_all_day: form.draws_all_day }
  try {
    if (form.id) await updateLotteryType(form.id, data)
    else await createLotteryType(data)
    ElMessage.success(form.id ? '彩种修改成功' : '彩种新增成功')
    dialogVisible.value = false
    await load()
  } finally { submitting.value = false }
}

async function remove(item: LotteryType) {
  await ElMessageBox.confirm(`确认删除彩种“${item.name}”？`, '删除确认', { type: 'warning' })
  await deleteLotteryType(item.id)
  ElMessage.success('删除成功')
  await load()
}

onMounted(load)
</script>

<style scoped>
.unit { margin-left: 10px; color: #606266; }
</style>
