<template>
  <section class="space-y-4">
    <div class="flex items-center justify-between"><div><h2 class="font-semibold">{{ admin ? '视频任务与账务' : '我的视频任务' }}</h2><p class="text-xs text-gray-500">后台定期更新状态。结果链接可能过期，完成后请及时下载保存。</p></div><button class="btn btn-secondary" :disabled="loading" @click="load">刷新列表</button></div>
    <p v-if="error" role="alert" class="text-sm text-red-600">{{ error }}</p>
    <div v-for="task in tasks" :key="task.id" class="rounded-lg border p-4 dark:border-dark-600">
      <div class="flex flex-wrap items-center justify-between gap-2"><strong>{{ task.model }} · {{ task.specification.resolution }} · {{ task.specification.duration }} 秒</strong><span>{{ states[task.state] || task.state }}</span></div>
      <p class="mt-2 text-xs text-gray-500">{{ new Date(task.created_at).toLocaleString() }} · {{ task.id }}<span v-if="admin"> · 用户 {{ task.user_id }} · 密钥 {{ task.api_key_id }}</span></p>
      <p class="mt-2 text-sm">${{ task.quote.total.toFixed(4) }} · {{ billing[task.billing_state] || task.billing_state }}</p>
      <p v-if="task.error" class="mt-2 text-sm text-amber-600">{{ task.error }}</p>
      <video v-if="safeURL(task.result?.task.content.url)" :src="safeURL(task.result?.task.content.url)" controls preload="none" class="mt-3 max-h-72 rounded-lg" />
      <div class="mt-3 flex flex-wrap gap-2"><button class="btn btn-secondary text-xs" :disabled="refreshing === task.id" @click="refresh(task.id)">更新状态</button><a v-if="safeURL(task.result?.task.content.url)" :href="safeURL(task.result?.task.content.url)" target="_blank" rel="noopener noreferrer" class="btn btn-secondary text-xs">打开／下载视频</a><button v-if="admin && task.state === 'uncertain' && task.billing_state === 'reserved'" class="btn btn-secondary text-xs" @click="releaseTask = task">核对并释放预占额度</button><button v-if="admin && ['failed','cancelled'].includes(task.state) && task.billing_state === 'charged'" class="btn btn-secondary text-xs" @click="refundTask = task">退还用户余额</button></div>
    </div>
    <p v-if="!tasks.length && !loading" class="py-8 text-center text-gray-500">暂无视频任务</p>
    <div class="flex justify-end gap-2"><button class="btn btn-secondary" :disabled="offset === 0 || loading" @click="offset -= 50; load()">上一页</button><button class="btn btn-secondary" :disabled="tasks.length < 50 || loading" @click="offset += 50; load()">下一页</button></div>
    <div v-if="refundTask" class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"><section role="dialog" aria-modal="true" aria-label="退款确认" class="max-w-lg space-y-4 rounded-xl bg-white p-6 dark:bg-dark-800"><h3 class="font-semibold">退还用户余额</h3><p>将向用户余额退回 ${{ refundTask.quote.total.toFixed(4) }}。此操作不代表供应商已退费，也不重置调用用量限额。订阅任务请在订阅管理中处理补偿。</p><div class="flex gap-3"><button class="btn btn-secondary" @click="refundTask = null">取消</button><button class="btn btn-primary" :disabled="loading" @click="refund">确认退款</button></div></section></div>
    <div v-if="releaseTask" class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"><section role="dialog" aria-modal="true" aria-label="核对任务" class="max-w-lg space-y-4 rounded-xl bg-white p-6 dark:bg-dark-800"><h3 class="font-semibold">核对上游是否已受理</h3><p>请先在供应商后台核实该任务未受理且未收费。确认后将释放 ${{ releaseTask.quote.total.toFixed(4) }} 预占额度。上游已受理的任务不能使用此操作。</p><div class="flex gap-3"><button class="btn btn-secondary" @click="releaseTask = null">返回</button><button class="btn btn-primary" :disabled="loading" @click="release">已核实未受理，释放额度</button></div></section></div>
  </section>
</template>
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { apiClient } from '@/api/client'
import type { VideoQuote } from '@/types/modelService'
import { extractApiErrorMessage } from '@/utils/apiError'
interface Task { id: string; user_id: number; api_key_id: number; model: string; state: string; billing_state: string; quote: VideoQuote; specification: { resolution: string; duration: number }; created_at: string; error?: string; result?: { task: { content: { url: string } } } }
const props = defineProps<{ admin?: boolean }>()
const tasks = ref<Task[]>([])
const loading = ref(false)
const refreshing = ref('')
const error = ref('')
const offset = ref(0)
const refundTask = ref<Task | null>(null)
const releaseTask = ref<Task | null>(null)
const states: Record<string, string> = { submitting: '提交中', uncertain: '受理情况待核对', queued: '排队中', running: '生成中', succeeded: '已完成', failed: '失败', cancelled: '已取消' }
const billing: Record<string, string> = { reserved: '额度已预占／待结算', charged: '已计费', released: '预占已释放', refunded: '已退还余额（调用用量保留）' }
function safeURL(raw?: string) { try { const url = new URL(raw || ''); return url.protocol === 'https:' ? url.href : undefined } catch { return undefined } }
function base() { return props.admin ? '/admin/media-tasks' : '/channels/media-tasks' }
async function load() { loading.value = true; error.value = ''; try { tasks.value = (await apiClient.get<Task[]>(base(), { params: { offset: offset.value } })).data } catch (e) { error.value = extractApiErrorMessage(e, '任务加载失败') } finally { loading.value = false } }
async function refresh(id: string) { refreshing.value = id; try { const { data } = await apiClient.post<Task>(`${base()}/${id}/refresh`); tasks.value = tasks.value.map(t => t.id === id ? data : t) } catch (e) { error.value = extractApiErrorMessage(e, '状态刷新失败') } finally { refreshing.value = '' } }
async function release() { if (!releaseTask.value) return; loading.value = true; try { await apiClient.post(`${base()}/${releaseTask.value.id}/release`, { confirmed_not_accepted: true }); releaseTask.value = null; await load() } catch (e) { error.value = extractApiErrorMessage(e, '释放失败') } finally { loading.value = false } }
async function refund() { if (!refundTask.value) return; loading.value = true; try { await apiClient.post(`${base()}/${refundTask.value.id}/refund`, { confirmed: true }); refundTask.value = null; await load() } catch (e) { error.value = extractApiErrorMessage(e, '退款失败') } finally { loading.value = false } }
defineExpose({ load })
onMounted(load)
</script>
