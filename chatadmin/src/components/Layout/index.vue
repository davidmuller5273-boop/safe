<template>
  <el-container class="layout-container">
    <el-aside :width="collapsed ? '64px' : '200px'" class="sidebar-container">
      <div class="logo">
        <img src="@/assets/logo.png" alt="管理后台" />
        <span v-if="!collapsed">管理后台</span>
      </div>
      <el-menu
        router
        :default-active="$route.path"
        :collapse="collapsed"
        :collapse-transition="false"
        background-color="#304156"
        text-color="#bfcbd9"
        active-text-color="#409eff"
      >
        <template v-if="activeSection === 'system'">
          <el-menu-item index="/dashboard"><el-icon><DataAnalysis /></el-icon><template #title>仪表盘</template></el-menu-item>
          <el-menu-item index="/permissions"><el-icon><Key /></el-icon><template #title>权限清单</template></el-menu-item>
          <el-menu-item index="/roles"><el-icon><Lock /></el-icon><template #title>角色管理</template></el-menu-item>
          <el-menu-item index="/admins"><el-icon><UserFilled /></el-icon><template #title>管理员管理</template></el-menu-item>
          <el-menu-item index="/system-config"><el-icon><Setting /></el-icon><template #title>系统配置</template></el-menu-item>
        </template>
        <template v-else>
          <el-menu-item index="/lottery/types"><el-icon><Tickets /></el-icon><template #title>彩种管理</template></el-menu-item>
          <el-menu-item index="/lottery/draw-records"><el-icon><List /></el-icon><template #title>开奖记录</template></el-menu-item>
        </template>
      </el-menu>
    </el-aside>

    <el-container>
      <el-header class="header-container">
        <div class="header-left">
          <el-icon class="collapse-icon" @click="collapsed = !collapsed">
            <Expand v-if="collapsed" /><Fold v-else />
          </el-icon>
          <nav class="top-menu-nav">
            <button type="button" class="top-menu-item" :class="{ active: activeSection === 'system' }" @click="openSection('system')">系统设置</button>
            <button type="button" class="top-menu-item" :class="{ active: activeSection === 'lottery' }" @click="openSection('lottery')">彩票管理</button>
          </nav>
        </div>
        <el-dropdown @command="handleCommand">
          <span class="user-info">
            <el-avatar :size="32">{{ store.user?.name?.slice(0, 1) || '管' }}</el-avatar>
            <span class="username">{{ store.user?.name || '管理员' }}</span>
            <el-tag v-if="store.user?.role_name" size="small" type="info" class="role-tag">{{ store.user.role_name }}</el-tag>
            <el-icon><ArrowDown /></el-icon>
          </span>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="password"><el-icon><Lock /></el-icon>修改密码</el-dropdown-item>
              <el-dropdown-item command="logout" divided>退出登录</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </el-header>
      <el-main class="main-container"><router-view /></el-main>
    </el-container>
    <el-dialog v-model="passwordDialogVisible" title="修改密码" width="440px" destroy-on-close @closed="resetPasswordForm">
      <el-form ref="passwordFormRef" :model="passwordForm" :rules="passwordRules" label-width="92px">
        <el-form-item label="原密码" prop="current_password">
          <el-input v-model="passwordForm.current_password" type="password" show-password autocomplete="current-password" placeholder="请输入原密码" />
        </el-form-item>
        <el-form-item label="新密码" prop="new_password">
          <el-input v-model="passwordForm.new_password" type="password" show-password autocomplete="new-password" placeholder="请输入6至72位新密码" />
        </el-form-item>
        <el-form-item label="确认密码" prop="confirm_password">
          <el-input v-model="passwordForm.confirm_password" type="password" show-password autocomplete="new-password" placeholder="请再次输入新密码" @keyup.enter="submitPassword" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="passwordDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="passwordSubmitting" @click="submitPassword">确认修改</el-button>
      </template>
    </el-dialog>
  </el-container>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { ArrowDown, DataAnalysis, Expand, Fold, Key, List, Lock, Setting, Tickets, UserFilled } from '@element-plus/icons-vue'
import { useUserStore } from '@/store/user'
import { changePassword } from '@/api'

const collapsed = ref(false)
const store = useUserStore()
const router = useRouter()
const route = useRoute()
const activeSection = computed(() => route.path.startsWith('/lottery/') ? 'lottery' : 'system')
const passwordDialogVisible = ref(false)
const passwordSubmitting = ref(false)
const passwordFormRef = ref<FormInstance>()
const passwordForm = reactive({ current_password: '', new_password: '', confirm_password: '' })
const validateConfirmPassword = (_rule: unknown, value: string, callback: (error?: Error) => void) => {
  if (value !== passwordForm.new_password) callback(new Error('两次输入的新密码不一致'))
  else callback()
}
const passwordRules: FormRules = {
  current_password: [{ required: true, message: '请输入原密码', trigger: 'blur' }],
  new_password: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 6, max: 72, message: '密码长度应为6至72位', trigger: 'blur' },
  ],
  confirm_password: [
    { required: true, message: '请再次输入新密码', trigger: 'blur' },
    { validator: validateConfirmPassword, trigger: 'blur' },
  ],
}

function openSection(section: 'system' | 'lottery') {
  router.push(section === 'lottery' ? '/lottery/types' : '/dashboard')
}

function handleCommand(command: string) {
  if (command === 'password') {
    passwordDialogVisible.value = true
  } else if (command === 'logout') {
    store.signOut()
    router.push('/login')
  }
}

function resetPasswordForm() {
  passwordFormRef.value?.resetFields()
  passwordForm.current_password = ''
  passwordForm.new_password = ''
  passwordForm.confirm_password = ''
}

async function submitPassword() {
  if (!passwordFormRef.value || !await passwordFormRef.value.validate().catch(() => false)) return
  passwordSubmitting.value = true
  try {
    await changePassword(passwordForm)
    ElMessage.success('密码修改成功，请重新登录')
    passwordDialogVisible.value = false
    store.signOut()
    router.push('/login')
  } finally {
    passwordSubmitting.value = false
  }
}
</script>

<style scoped lang="scss">
.layout-container { height: 100vh; }
.sidebar-container { background-color: #304156; transition: width 0.3s; }
.logo { height: 60px; display: flex; align-items: center; justify-content: center; color: #fff; font-size: 18px; font-weight: 700; white-space: nowrap; overflow: hidden; }
.logo img { width: 32px; height: 32px; margin-right: 8px; border-radius: 50%; }
.el-menu { border-right: none; }
.header-container { height: 60px; padding: 0 20px; display: flex; align-items: center; justify-content: space-between; background: #fff; border-bottom: 1px solid #e4e7ed; }
.header-left { display: flex; align-items: center; gap: 24px; flex: 1; }
.collapse-icon { flex-shrink: 0; color: #606266; font-size: 20px; cursor: pointer; }
.collapse-icon:hover { color: #409eff; }
.top-menu-nav { display: flex; align-items: center; gap: 8px; }
.top-menu-item { padding: 8px 20px; border: none; border-radius: 4px; background: transparent; color: #606266; font-size: 15px; cursor: pointer; }
.top-menu-item.active, .top-menu-item:hover { color: #409eff; font-weight: 600; background: #ecf5ff; }
.user-info { display: flex; align-items: center; cursor: pointer; outline: none; }
.username { margin: 0 8px; font-size: 14px; }
.role-tag { margin-right: 6px; }
.main-container { padding: 20px; background: #f0f2f5; }
</style>
