<template>
  <div>
    <div class="page-head"><h2>系统配置</h2></div>
    <el-card v-loading="loading" shadow="never" class="config-card">
      <template #header>
        <div class="card-header">
          <span>SafeW 机器人</span>
          <el-tag :type="config.safew_bot_ready ? 'success' : 'info'">
            {{ config.safew_bot_ready ? '已就绪' : '未完整配置' }}
          </el-tag>
        </div>
      </template>
      <el-form ref="formRef" :model="form" :rules="rules" label-width="150px">
        <el-form-item label="机器人 Token" prop="token">
          <el-input v-model.trim="form.token" type="password" show-password autocomplete="new-password" placeholder="请输入新的 SafeW 机器人 Token" />
          <div v-if="config.safew_bot_token_configured" class="config-hint">
            当前 Token：{{ config.safew_bot_token_masked }}。留空表示保留现有 Token。
          </div>
        </el-form-item>
        <div v-for="(_chatID, index) in form.chat_ids" :key="index" class="chat-id-row">
          <el-form-item
            :label="index === 0 ? '群聊 ID' : ''"
            :prop="`chat_ids.${index}`"
            :rules="[{ required: true, message: '请输入 SafeW 群聊 ID', trigger: 'blur' }]"
          >
            <div class="chat-id-input">
              <el-input v-model.trim="form.chat_ids[index]" placeholder="请输入机器人所在的 SafeW 群聊 ID" />
              <el-button v-if="form.chat_ids.length > 1" type="danger" plain @click="removeChatID(index)">删除</el-button>
            </div>
            <div v-if="index === 0" class="config-hint">新的开奖记录将自动发送到所有已配置群聊。</div>
          </el-form-item>
        </div>
        <el-form-item>
          <el-button plain @click="addChatID">添加群聊</el-button>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="submitting" @click="save">保存配置</el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { getSystemConfig, updateSafeWBot } from '@/api'
import type { SystemConfig } from '@/types'

const loading = ref(false)
const submitting = ref(false)
const formRef = ref<FormInstance>()
const form = reactive({ token: '', chat_ids: [''] })
const config = reactive<SystemConfig>({ safew_bot_token_configured: false, safew_bot_token_masked: '', safew_chat_ids: [], safew_bot_ready: false })
const rules: FormRules = {
  token: [{ validator: (_rule, value, callback) => !config.safew_bot_token_configured && !value ? callback(new Error('请输入 SafeW 机器人 Token')) : callback(), trigger: 'blur' }],
}

async function load() {
  loading.value = true
  try {
    Object.assign(config, (await getSystemConfig()).data)
    form.chat_ids = config.safew_chat_ids.length ? [...config.safew_chat_ids] : ['']
  } finally { loading.value = false }
}

async function save() {
  if (!formRef.value || !await formRef.value.validate().catch(() => false)) return
  submitting.value = true
  try {
    const chatIDs = form.chat_ids.map(value => value.trim()).filter(Boolean)
    if (new Set(chatIDs).size !== chatIDs.length) {
      ElMessage.warning('群聊 ID 不能重复')
      return
    }
    await updateSafeWBot({ token: form.token, chat_ids: chatIDs })
    form.token = ''
    formRef.value.clearValidate()
    ElMessage.success('SafeW 机器人配置保存成功')
    await load()
  } finally { submitting.value = false }
}

onMounted(load)

function addChatID() {
  form.chat_ids.push('')
}

function removeChatID(index: number) {
  form.chat_ids.splice(index, 1)
}
</script>

<style scoped>
.config-card { max-width: 760px; }
.card-header { display: flex; align-items: center; justify-content: space-between; }
.config-hint { margin-top: 8px; color: #909399; font-size: 13px; line-height: 1.5; }
.chat-id-input { display: flex; width: 100%; gap: 10px; }
.chat-id-input .el-input { flex: 1; }
</style>
