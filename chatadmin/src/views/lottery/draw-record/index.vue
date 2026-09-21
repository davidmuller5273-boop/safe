<template>
  <div>
    <div class="page-head">
      <h2>开奖记录</h2>
      <el-button :loading="loading" @click="load">刷新</el-button>
    </div>

    <el-card shadow="never" class="filter-card">
      <el-form :inline="true" :model="filters">
        <el-form-item label="彩种">
          <el-select v-model="filters.lottery_type_id" clearable placeholder="全部彩种" class="lottery-select">
            <el-option v-for="item in lotteryTypes" :key="item.id" :label="`${item.name} (${item.symbol})`" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="期号">
          <el-input v-model.trim="filters.issue_number" clearable placeholder="输入期号查询" @keyup.enter="search" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="search">查询</el-button>
          <el-button @click="reset">重置</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <el-card shadow="never" class="table-card">
      <el-table v-loading="loading" :data="items" border stripe>
        <el-table-column label="彩种" min-width="160">
          <template #default="scope">
            <div class="lottery-name">{{ scope.row.lottery_type?.name || '-' }}</div>
            <div class="lottery-symbol">{{ scope.row.lottery_type?.symbol || '-' }}</div>
          </template>
        </el-table-column>
        <el-table-column prop="issue_number" label="期号" min-width="140" />
        <el-table-column label="开奖结果" min-width="390">
          <template #default="scope">
            <div class="draw-result">
              <span v-for="(number, index) in parseResult(scope.row.draw_result)" :key="`${scope.row.id}-${index}`" class="number-ball">{{ number }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="开奖时间" min-width="180">
          <template #default="scope">{{ formatTimestamp(scope.row.draw_timestamp) }}</template>
        </el-table-column>
        <el-table-column prop="next_issue_number" label="下一期期号" min-width="140" />
        <el-table-column label="下期开奖时间" min-width="180">
          <template #default="scope">{{ formatTimestamp(scope.row.next_draw_timestamp) }}</template>
        </el-table-column>
        <template #empty>暂无开奖记录</template>
      </el-table>

      <div class="pagination-wrap">
        <el-pagination
          v-model:current-page="pagination.page"
          v-model:page-size="pagination.pageSize"
          :page-sizes="[20, 50, 100]"
          :total="pagination.total"
          layout="total, sizes, prev, pager, next, jumper"
          @current-change="load"
          @size-change="handleSizeChange"
        />
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { getDrawRecords, getLotteryTypes } from '@/api'
import type { DrawRecord, LotteryType } from '@/types'

const loading = ref(false)
const items = ref<DrawRecord[]>([])
const lotteryTypes = ref<LotteryType[]>([])
const filters = reactive<{ lottery_type_id?: number; issue_number: string }>({ lottery_type_id: undefined, issue_number: '' })
const pagination = reactive({ page: 1, pageSize: 20, total: 0 })
const beijingFormatter = new Intl.DateTimeFormat('zh-CN', {
  timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit',
  hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false,
})

async function load() {
  loading.value = true
  try {
    const response = await getDrawRecords({
      page: pagination.page,
      page_size: pagination.pageSize,
      lottery_type_id: filters.lottery_type_id,
      issue_number: filters.issue_number || undefined,
    })
    items.value = response.data.items
    pagination.total = response.data.total
  } finally { loading.value = false }
}

async function loadLotteryTypes() {
  lotteryTypes.value = (await getLotteryTypes()).data
}

function search() { pagination.page = 1; load() }
function reset() {
  filters.lottery_type_id = undefined
  filters.issue_number = ''
  pagination.page = 1
  load()
}
function handleSizeChange() { pagination.page = 1; load() }

function parseResult(value: string): string[] {
  try {
    const result = JSON.parse(value)
    return Array.isArray(result) ? result.map(String) : [value]
  } catch {
    return value.split(',').map(item => item.trim()).filter(Boolean)
  }
}

function formatTimestamp(value: number): string {
  if (!value) return '-'
  return beijingFormatter.format(new Date(value * 1000)).replace(/\//g, '-')
}

onMounted(async () => {
  await Promise.all([loadLotteryTypes(), load()])
})
</script>

<style scoped>
.filter-card { margin-bottom: 16px; }
.filter-card :deep(.el-card__body) { padding-bottom: 2px; }
.lottery-select { width: 230px; }
.table-card :deep(.el-card__body) { padding: 0; }
.lottery-name { font-weight: 500; color: #303133; }
.lottery-symbol { margin-top: 3px; color: #909399; font-size: 12px; }
.draw-result { display: flex; flex-wrap: wrap; gap: 6px; }
.number-ball { display: inline-flex; align-items: center; justify-content: center; width: 30px; height: 30px; border-radius: 50%; background: #409eff; color: #fff; font-size: 12px; font-weight: 700; box-shadow: 0 2px 5px rgb(64 158 255 / 28%); }
.pagination-wrap { display: flex; justify-content: flex-end; padding: 16px; }
</style>
