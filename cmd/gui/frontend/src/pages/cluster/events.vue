<script lang="ts" setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessagesStore } from '@/stores/snackbar'
import { GetClusterEventLogPage, ClearClusterEventLog } from '../../../bindings/bilibili-ticket-golang/cmd/gui/cluster_service/clusterservice'

const { t } = useI18n()
const messages = useMessagesStore()

interface ClusterEvent {
    time: string
    kind: string
    workerId: string
    stage: string
    message: string
    orderId?: string
    attemptId?: string
    code: number
    retryable: boolean
}

const events = ref<ClusterEvent[]>([])
const totalEvents = ref(0)
const page = ref(1)
const pageSize = ref(20)
const loading = ref(true)
const clearing = ref(false)
const clearDialog = ref(false)
let timer: ReturnType<typeof setInterval> | null = null
let searchTimer: ReturnType<typeof setTimeout> | null = null
let latestRequest = 0

async function load() {
    const request = ++latestRequest
    loading.value = true
    try {
        const resp = await GetClusterEventLogPage(page.value, pageSize.value, searchText.value.trim())
        if (request !== latestRequest) return
        totalEvents.value = Number(resp.total || 0)
        const lastPage = Math.max(1, Math.ceil(totalEvents.value / pageSize.value))
        if (page.value > lastPage) {
            page.value = lastPage
            return
        }
        events.value = (resp.events || []) as ClusterEvent[]
    } catch { /* silent */ }
    finally {
        if (request === latestRequest) loading.value = false
    }
}

async function clearEvents() {
    clearDialog.value = false
    clearing.value = true
    try {
        const deleted = await ClearClusterEventLog()
        events.value = []
        totalEvents.value = 0
        page.value = 1
        await load()
        messages.add({ text: t('events.clearSuccess', { count: deleted ?? 0 }), color: 'success' })
    } catch (e: any) {
        messages.add({ text: t('events.clearFailed', { error: String(e) }), color: 'error' })
    }
    clearing.value = false
}

onMounted(async () => {
    await load()
    timer = setInterval(load, 3000)
})
onUnmounted(() => {
    if (timer) { clearInterval(timer); timer = null }
    if (searchTimer) { clearTimeout(searchTimer); searchTimer = null }
})

function fmtTime(ts: any): string {
    try {
        const d = ts instanceof Date ? ts : new Date(ts)
        if (isNaN(d.getTime())) return String(ts ?? '')
        const yy = String(d.getFullYear())
        const mo = String(d.getMonth() + 1).padStart(2, '0')
        const dd = String(d.getDate()).padStart(2, '0')
        const hh = String(d.getHours()).padStart(2, '0')
        const mi = String(d.getMinutes()).padStart(2, '0')
        const ss = String(d.getSeconds()).padStart(2, '0')
        const ms = String(d.getMilliseconds()).padStart(3, '0')
        return `${yy}-${mo}-${dd} ${hh}:${mi}:${ss}.${ms}`
    } catch { return String(ts ?? '') }
}

function kindLabel(k: string): string {
    const m: Record<string, string> = {
        worker_connected: t('events.kindWorkerConnected'),
        worker_disconnected: t('events.kindWorkerDisconnected'),
        worker_healthy: t('events.kindWorkerHealthy'),
        worker_unhealthy: t('events.kindWorkerUnhealthy'),
        task_completed: t('events.kindTaskCompleted'),
        task_failed: t('events.kindTaskFailed'),
        task_superseded: t('events.kindTaskSuperseded'),
        task_stopped: t('events.kindTaskStopped'),
        heartbeat_timeout: t('events.kindHeartbeatTimeout'),
        heartbeat_latency: t('events.kindHeartbeatLatency'),
        worker_info: t('events.kindWorkerInfo'),
        dispatch_info: t('events.kindDispatchInfo'),
        dispatch_warning: t('events.kindDispatchWarning'),
    }
    return m[k] || k
}

function kindColor(k: string): string {
    switch (k) {
        case 'worker_connected':
        case 'task_completed':
        case 'worker_healthy': return 'success'
        case 'worker_disconnected':
        case 'task_failed':
        case 'worker_unhealthy': return 'error'
        case 'task_superseded': return 'info'
        case 'task_stopped': return 'grey'
        case 'heartbeat_timeout':
        case 'heartbeat_latency':
        case 'dispatch_warning': return 'warning'
        case 'dispatch_info': return 'info'
        default: return ''
    }
}

// ── Table ────────────────────────────────────────────────────
const searchText = ref('')
const pageCount = computed(() => Math.max(1, Math.ceil(totalEvents.value / pageSize.value)))
const rangeStart = computed(() => totalEvents.value === 0 ? 0 : (page.value - 1) * pageSize.value + 1)
const rangeEnd = computed(() => Math.min(page.value * pageSize.value, totalEvents.value))
const pageSizeOptions = [10, 20, 50]

watch(page, () => { void load() })
watch(pageSize, () => {
    if (page.value !== 1) page.value = 1
    else void load()
})
watch(searchText, () => {
    if (searchTimer) clearTimeout(searchTimer)
    searchTimer = setTimeout(() => {
        if (page.value !== 1) page.value = 1
        else void load()
    }, 300)
})

const tableHeaders = computed(() => [
    { title: t('events.colTime'), key: 'time', width: 170, sortable: false },
    { title: t('events.colWorker'), key: 'workerId', minWidth: 140, sortable: false },
    { title: t('events.colStage'), key: 'stage', width: 70, sortable: false },
    { title: t('events.colType'), key: 'kind', width: 100, sortable: false },
    { title: t('events.colMessage'), key: 'message', sortable: false },
])
</script>

<template>
    <v-container>
        <div class="page-title-bar">
            <h1 class="page-title">{{ t('events.title') }}</h1>
        </div>

        <!-- Event log table -->
        <v-card elevation="2">
            <v-card-item class="py-2 px-4">
                <template #title>
                    <span class="text-subtitle-2">{{ t('events.feedTitle') }}</span>
                    <span class="text-caption text-medium-emphasis ml-2">({{ totalEvents }} {{ t('events.entries')
                    }})</span>
                </template>
                <template #append>
                    <v-btn size="x-small" variant="text" color="error" :loading="clearing" prepend-icon="mdi-delete"
                        class="mr-2" @click="clearDialog = true">
                        {{ t('events.clear') }}
                    </v-btn>
                    <v-btn size="x-small" variant="text" :loading="loading" icon="mdi-refresh" @click="load" />
                </template>
            </v-card-item>

            <v-text-field v-model="searchText" density="compact" variant="outlined" hide-details
                :placeholder="t('events.searchPlaceholder')" prepend-inner-icon="mdi-magnify" clearable
                class="mx-4 mt-2" />

            <v-data-table v-if="events.length > 0" :headers="tableHeaders" :items="events"
                :items-per-page="-1" hide-default-footer density="compact" hover class="events-table"
                :loading="loading">
                <template #item.time="{ item }">
                    <span class="font-monospace text-caption text-no-wrap">{{ fmtTime(item.time) }}</span>
                </template>
                <template #item.workerId="{ item }">
                    <span class="font-monospace text-caption worker-id-full">{{ item.workerId || '—'
                        }}</span>
                </template>
                <template #item.stage="{ item }">
                    <span class="font-monospace text-caption font-weight-bold"
                        :class="kindColor(item.kind) ? 'text-' + kindColor(item.kind) : ''">{{ item.stage }}</span>
                </template>
                <template #item.kind="{ item }">
                    <span class="text-caption" :class="kindColor(item.kind) ? 'text-' + kindColor(item.kind) : ''">{{
                        kindLabel(item.kind) }}</span>
                </template>
                <template #item.message="{ item }">
                    <span class="text-caption" :class="kindColor(item.kind) ? 'text-' + kindColor(item.kind) : ''">{{
                        item.message }}</span>
                </template>
            </v-data-table>

            <template v-if="totalEvents > 0">
                <v-divider />
                <div class="events-pagination px-4 py-2">
                    <span class="text-caption text-medium-emphasis text-no-wrap">
                        {{ rangeStart }}-{{ rangeEnd }} / {{ totalEvents }}
                    </span>
                    <v-spacer />
                    <span class="text-caption text-medium-emphasis text-no-wrap">
                        {{ t('$vuetify.dataFooter.itemsPerPageText') }}
                    </span>
                    <v-select v-model="pageSize" :items="pageSizeOptions" density="compact" variant="outlined"
                        hide-details class="page-size-select" />
                    <v-pagination v-model="page" :length="pageCount" :total-visible="7" density="compact"
                        rounded="circle" />
                </div>
            </template>

            <div v-if="!loading && totalEvents === 0" class="text-center py-6">
                <v-icon size="36" class="mb-2" color="medium-emphasis">mdi-text-box-outline</v-icon>
                <p class="text-caption text-medium-emphasis">{{ t('events.empty') }}</p>
            </div>

            <div v-if="loading && events.length === 0" class="text-center py-6">
                <v-progress-circular indeterminate color="primary" size="28" />
                <p class="text-caption text-medium-emphasis mt-2">{{ t('events.loading') }}</p>
            </div>

        </v-card>

        <v-dialog v-model="clearDialog" max-width="420">
            <v-card>
                <v-card-title>{{ t('events.clear') }}</v-card-title>
                <v-card-text>{{ t('events.clearConfirm') }}</v-card-text>
                <v-card-actions>
                    <v-spacer />
                    <v-btn variant="text" @click="clearDialog = false">{{ t('common.cancel') }}</v-btn>
                    <v-btn color="error" :loading="clearing" @click="clearEvents">{{ t('events.clear') }}</v-btn>
                </v-card-actions>
            </v-card>
        </v-dialog>
    </v-container>
</template>

<style scoped>
.worker-id-full {
    white-space: normal;
    overflow-wrap: anywhere;
    word-break: break-word;
}
</style>

<style scoped>
.events-table :deep(td) {
    font-family: 'Cascadia Code', 'Fira Code', 'Consolas', monospace;
    font-size: 11px;
}

.events-table :deep(table) {
    min-width: 1180px;
}

.events-table :deep(th),
.events-table :deep(td) {
    white-space: nowrap;
}

.events-pagination {
    display: flex;
    align-items: center;
    gap: 12px;
    min-height: 52px;
}

.page-size-select {
    flex: 0 0 84px;
    max-width: 84px;
}

@media (max-width: 720px) {
    .events-pagination {
        flex-wrap: wrap;
    }

    .events-pagination :deep(.v-pagination) {
        width: 100%;
    }
}
</style>
