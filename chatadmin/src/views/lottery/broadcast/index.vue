<template>
  <div v-loading="loading">
    <div class="page-head">
      <h2>开奖播报</h2>
      <el-button @click="load">刷新</el-button>
    </div>

    <el-card shadow="never" class="block">
      <template #header>总开关</template>
      <div class="switches">
        <span><el-switch v-model="data.query_enabled" @change="(v: boolean) => saveFlag('query_enabled', v)" /> 启用开奖查询与轮询</span>
        <span><el-switch v-model="data.broadcast_enabled" @change="(v: boolean) => saveFlag('broadcast_enabled', v)" /> 新期开奖自动播报（并置顶）</span>
        <span><el-switch v-model="data.with_ads" @change="(v: boolean) => saveFlag('with_ads', v)" /> 播报附带本群前后广告</span>
      </div>
      <p class="tip">每 5 秒轮询，开奖前 2 分钟至开奖后 45 分钟为密集窗口；只有出现新一期且该彩种开关为开时，才推送给已订阅的群（约 3 秒内送达并自动置顶，机器人需为群管理员并有置顶权限）。</p>
    </el-card>

    <el-card shadow="never" class="block">
      <template #header>彩种播报开关</template>
      <div class="switches">
        <span v-for="g in data.games" :key="g.code"><el-switch v-model="g.enabled" @change="(v: boolean) => saveGame(g.code, v)" /> {{ g.name }} <small>{{ g.source }}</small></span>
      </div>
    </el-card>

    <el-card shadow="never" class="block">
      <template #header>最新开奖</template>
      <el-table :data="data.latest" border size="small">
        <el-table-column prop="game_name" label="彩种" width="120" />
        <el-table-column prop="issue" label="期号" width="130" />
        <el-table-column prop="draw_time" label="开奖时间" width="170" />
        <el-table-column prop="numbers" label="号码" min-width="220" />
        <el-table-column prop="source" label="数据源" width="140" />
        <template #empty>暂无（有群订阅后开始轮询）</template>
      </el-table>
    </el-card>

    <el-card shadow="never" class="block">
      <template #header>群订阅（{{ data.subscriptions.length }}）</template>
      <el-table :data="data.subscriptions" border size="small">
        <el-table-column label="群" min-width="160"><template #default="s">{{ s.row.group_title || '(无标题)' }}</template></el-table-column>
        <el-table-column prop="chat_id" label="群ID" width="170" />
        <el-table-column prop="label" label="订阅范围" width="120" />
        <el-table-column prop="created_by" label="订阅人" width="130" />
        <el-table-column label="操作" width="90"><template #default="s"><el-button link type="danger" @click="removeSub(s.row)">删除</el-button></template></el-table-column>
        <template #empty>暂无订阅（群内发送 /订阅开奖 或点 /菜单 按钮）</template>
      </el-table>
    </el-card>

    <el-card shadow="never" class="block">
      <template #header>数据源状态</template>
      <el-table :data="data.sources" border size="small">
        <el-table-column prop="label" label="数据源" width="160" />
        <el-table-column prop="last_checked_at" label="最近检查" width="200" />
        <el-table-column prop="last_success_at" label="最近成功" width="200" />
        <el-table-column label="状态" min-width="200"><template #default="s"><el-tag v-if="!s.row.last_error" type="success">正常</el-tag><span v-else class="err">{{ s.row.last_error }}</span></template></el-table-column>
      </el-table>
    </el-card>

    <el-card shadow="never" class="block">
      <template #header>最近播报</template>
      <el-table :data="data.outbox" border size="small">
        <el-table-column prop="created_at" label="时间" width="200" />
        <el-table-column prop="chat_id" label="群ID" width="170" />
        <el-table-column label="彩种" width="120"><template #default="s">{{ s.row.game_name || s.row.game_code }}</template></el-table-column>
        <el-table-column prop="issue" label="期号" width="130" />
        <el-table-column label="状态" width="90"><template #default="s"><el-tag :type="s.row.status === 'sent' ? 'success' : s.row.status === 'failed' ? 'danger' : 'warning'">{{ statusText[s.row.status] || s.row.status }}</el-tag></template></el-table-column>
        <el-table-column label="置顶" width="70"><template #default="s">{{ s.row.pinned ? '✅' : '—' }}</template></el-table-column>
        <el-table-column prop="last_error" label="备注" min-width="200" />
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { deleteLotteryBroadcastSubscription, getLotteryBroadcast, updateLotteryBroadcastSwitches } from '@/api'
import type { LotteryBroadcastOverview, LotteryBroadcastSubscription } from '@/types'

const loading = ref(false)
const statusText: Record<string, string> = { sent: '已发送', pending: '待发送', failed: '失败' }
const data = reactive<LotteryBroadcastOverview>({ query_enabled: true, broadcast_enabled: true, with_ads: true, games: [], latest: [], sources: [], subscriptions: [], outbox: [] })

async function load() {
  loading.value = true
  try { Object.assign(data, (await getLotteryBroadcast()).data) } finally { loading.value = false }
}

async function saveFlag(key: 'query_enabled' | 'broadcast_enabled' | 'with_ads', value: boolean) {
  try { await updateLotteryBroadcastSwitches({ [key]: value }); ElMessage.success('已保存') } catch { data[key] = !value }
}

async function saveGame(code: string, value: boolean) {
  try { await updateLotteryBroadcastSwitches({ games: { [code]: value } }); ElMessage.success('已保存') } catch { await load() }
}

async function removeSub(row: LotteryBroadcastSubscription) {
  await ElMessageBox.confirm(`确定删除群 ${row.group_title || row.chat_id} 的「${row.label}」订阅？`, '提示', { type: 'warning' })
  await deleteLotteryBroadcastSubscription({ chat_id: row.chat_id, selector: row.selector })
  ElMessage.success('已删除')
  await load()
}

onMounted(load)
</script>

<style scoped>
.page-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.block { margin-bottom: 16px; }
.switches { display: flex; flex-wrap: wrap; gap: 12px 28px; }
.switches small { color: #909399; }
.tip { color: #909399; font-size: 12px; margin: 12px 0 0; }
.err { color: #f56c6c; }
</style>
