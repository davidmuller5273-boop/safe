<template>
  <div class="permission-container">
    <el-card shadow="never">
      <template #header>
        <div class="card-header">
          <span>权限清单</span>
          <span class="tips">以下为系统内置的所有权限标识，可在角色管理中分配给角色</span>
        </div>
      </template>
      <div v-for="(permissions, module) in modules" :key="module" class="permission-module">
        <div class="module-title">{{ moduleLabels[module] || module }}</div>
        <el-space wrap>
          <el-tag v-for="permission in permissions" :key="permission.id" type="info" effect="plain" class="permission-tag">
            <span class="permission-code">{{ permission.code }}</span>
            <span class="permission-name">{{ permission.name }}</span>
          </el-tag>
        </el-space>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { getPermissions } from '@/api'
import type { Permission } from '@/types'

const items = ref<Permission[]>([])
const moduleLabels: Record<string, string> = { dashboard: '仪表盘', admin: '管理员管理', role: '角色管理', permission: '权限管理' }
const modules = computed(() => items.value.reduce<Record<string, Permission[]>>((result, permission) => {
  const module = permission.code.split(/[:.]/)[0]
  ;(result[module] ||= []).push(permission)
  return result
}, {}))

onMounted(async () => { items.value = (await getPermissions()).data })
</script>

<style scoped lang="scss">
.card-header { display: flex; align-items: center; justify-content: space-between; }
.tips { color: #909399; font-size: 12px; }
.permission-module { margin-bottom: 24px; }
.permission-module:last-child { margin-bottom: 0; }
.module-title { margin-bottom: 12px; padding-left: 4px; border-left: 3px solid #409eff; color: #303133; font-size: 14px; font-weight: 600; }
.permission-tag { display: inline-flex; align-items: center; gap: 6px; }
.permission-code { color: #409eff; font-family: Consolas, monospace; font-size: 12px; }
.permission-name { color: #606266; font-size: 12px; }
</style>
