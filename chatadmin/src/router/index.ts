import { createRouter, createWebHistory } from 'vue-router'
import { useUserStore } from '@/store/user'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: () => import('@/views/login/index.vue'), meta: { public: true } },
    {
      path: '/',
      component: () => import('@/components/Layout/index.vue'),
      redirect: '/dashboard',
      children: [
        { path: 'dashboard', component: () => import('@/views/dashboard/index.vue'), meta: { title: '控制台' } },
        { path: 'admins', component: () => import('@/views/admin/index.vue'), meta: { title: '管理员' } },
        { path: 'roles', component: () => import('@/views/role/index.vue'), meta: { title: '角色管理' } },
        { path: 'permissions', component: () => import('@/views/permission/index.vue'), meta: { title: '权限列表' } },
        { path: 'system-config', component: () => import('@/views/system-config/index.vue'), meta: { title: '系统配置' } },
        { path: 'lottery/types', component: () => import('@/views/lottery/type/index.vue'), meta: { title: '彩种管理' } },
        { path: 'lottery/draw-records', component: () => import('@/views/lottery/draw-record/index.vue'), meta: { title: '开奖记录' } },
      ],
    },
  ],
})

router.beforeEach(async (to) => {
  if (to.meta.public) return true
  const store = useUserStore()
  if (!store.token) return '/login'
  if (!store.user && !await store.load()) return '/login'
  return true
})

export default router
