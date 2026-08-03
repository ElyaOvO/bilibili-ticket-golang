<script lang="ts" setup>
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { GetWorkerListSnapshot, TestWorkerCaptcha, TestAllWorkersCaptcha } from '../../bindings/bilibili-ticket-golang/cmd/gui/cluster_service/clusterservice'
import { GetCaptchaStatus, HasCaptchaDLL, HasCaptchaSolver, TestCaptchaSolver } from '../../bindings/bilibili-ticket-golang/cmd/gui/app'

const { t } = useI18n()

// ---- employer state ----
const employerDLL = ref(false)
const employerSolver = ref(false)
const employerTesting = ref(false)
const employerResult = ref<{ success?: boolean; elapsed?: string; error?: string; type?: string } | null>(null)
const captchaStatus = ref<{ loaded?: boolean; version?: string; gitCommit?: string; error?: string }>({})

// ---- worker state ----
const workers = ref<{ id: string; name: string }[]>([])
const workerTesting = ref(false)
const workerResults = ref<any[]>([])

// ---- refresh employer status ----
async function refresh() {
    try {
        captchaStatus.value = (await GetCaptchaStatus()) || {}
        employerDLL.value = !!captchaStatus.value.loaded
    } catch {
        captchaStatus.value = {}
        try { employerDLL.value = !!(await HasCaptchaDLL()) } catch { employerDLL.value = false }
    }
    try { employerSolver.value = !!(await HasCaptchaSolver()) } catch { employerSolver.value = false }
    try {
        const snap = await GetWorkerListSnapshot()
        workers.value = ((snap as any)?.workers || []).filter((w: any) => w.type !== 'local')
    } catch { workers.value = [] }
}

function captchaVersionText() {
    const version = captchaStatus.value.version || ''
    const commit = captchaStatus.value.gitCommit || ''
    if (version && commit) return `${version}+${commit}`
    return version || commit || '-'
}

// ---- employer test ----
async function testEmployer() {
    employerTesting.value = true; employerResult.value = null
    try {
        employerResult.value = await TestCaptchaSolver()
    } catch (e: any) {
        employerResult.value = { success: false, error: String(e) }
    } finally { employerTesting.value = false }
}

// ---- worker tests ----
async function testWorker(id: string) {
    workerTesting.value = true; workerResults.value = []
    try {
        workerResults.value = [await TestWorkerCaptcha(id)]
    } catch (e: any) {
        workerResults.value = [{ id, success: false, error: String(e) }]
    } finally { workerTesting.value = false }
}

async function testAll() {
    workerTesting.value = true; workerResults.value = []
    try {
        workerResults.value = (await TestAllWorkersCaptcha()) || []
    } catch { workerResults.value = [] } finally { workerTesting.value = false }
}

onMounted(refresh)
</script>

<template>
    <div>
        <div class="page-title-bar">
            <h1 class="page-title">{{ t('captcha.title') }}</h1>
            <v-spacer />
        </div>

        <!-- 雇主端 -->
        <v-card variant="outlined" :loading="employerTesting">
            <v-card-title class="text-body-1 py-3 px-4 d-flex align-center">
                <v-icon start size="20" color="primary">mdi-monitor</v-icon>
                {{ t('captcha.employerDLL') }}
                <v-spacer />
                <v-btn size="x-small" variant="text" density="compact" icon="mdi-refresh"
                    :aria-label="t('common.refresh')" @click="refresh" />
            </v-card-title>
            <v-divider />
            <v-card-text class="pa-4">

                <!-- 状态 -->
                <div class="captcha-status-grid mb-4">
                    <div class="captcha-library-status"
                        :class="employerDLL ? 'captcha-library-status--success' : 'captcha-library-status--error'">
                        <v-icon :color="employerDLL ? 'success' : 'error'" size="20" class="captcha-status-icon">
                            {{ employerDLL ? 'mdi-check-circle' : 'mdi-close-circle' }}
                        </v-icon>
                        <span class="text-body-2 captcha-status-message">{{ employerDLL ? t('captcha.dllFound', {
                            Version: captchaVersionText() }) :
                            (captchaStatus.error || t('captcha.dllNotFound')) }}</span>
                    </div>
                    <v-chip size="small" :color="employerSolver ? 'success' : 'error'" variant="tonal"
                        class="captcha-solver-chip">
                        <v-icon start size="16">
                            {{ employerSolver ? 'mdi-puzzle-check' : 'mdi-puzzle-remove' }}
                        </v-icon>
                        {{ employerSolver ? t('captcha.solverInstalled') : t('captcha.solverNotInstalled') }}
                    </v-chip>
                </div>

                <!-- 测试按钮 + 结果 -->
                <div class="captcha-test-row">
                    <v-btn color="primary" variant="tonal" size="small" :loading="employerTesting"
                        :disabled="!employerDLL" @click="testEmployer">
                        <v-icon start size="18">mdi-play</v-icon>
                        {{ t('captcha.test') }}
                    </v-btn>
                    <template v-if="employerResult">
                        <v-icon :color="employerResult.success ? 'success' : 'error'" size="20">
                            {{ employerResult.success ? 'mdi-check' : 'mdi-alert' }}
                        </v-icon>
                        <span class="text-body-2 text-medium-emphasis captcha-test-result">
                            {{ employerResult.success ? `${employerResult.type} · ${employerResult.elapsed}` :
                                employerResult.error }}
                        </span>
                    </template>
                </div>
            </v-card-text>
        </v-card>

        <!-- Worker 端 -->
        <v-card variant="outlined" class="mt-4" :loading="workerTesting && workers.length > 0">
            <v-card-title class="text-body-1 py-3 px-4">
                <v-icon start size="20" color="secondary">mdi-server-network</v-icon>
                {{ t('captcha.workerTest') }}
            </v-card-title>
            <v-divider />

            <template v-if="workers.length === 0">
                <v-card-text class="pa-4 text-body-2 text-medium-emphasis">
                    无远程 Worker
                </v-card-text>
            </template>

            <template v-else>
                <v-card-text class="pa-4">

                    <!-- Worker 选择按钮 -->
                    <div class="d-flex flex-wrap align-center ga-2 mb-4">
                        <v-btn v-for="w in workers" :key="w.id" size="small" variant="outlined" color="primary"
                            rounded="pill" :disabled="workerTesting" class="worker-id-button"
                            @click="testWorker(w.id)">
                            {{ w.name || w.id }}
                        </v-btn>
                        <v-btn size="small" variant="tonal" color="secondary" rounded="pill" :disabled="workerTesting"
                            @click="testAll">
                            {{ t('captcha.testAll') }}
                        </v-btn>
                    </div>

                    <!-- 测试结果 -->
                    <template v-for="r in workerResults" :key="r.id || r.workerId || '?'">
                        <v-alert :type="r.success ? 'success' : 'error'" variant="tonal" density="compact" class="mb-2">
                            <template #prepend>
                                <v-icon :icon="r.success ? 'mdi-check' : 'mdi-alert'" size="18" />
                            </template>
                            <strong class="worker-id-full">{{ r.id || r.workerId }}</strong>
                            <template v-if="r.success">
                                — {{ r.elapsed }} ({{ r.type }})
                            </template>
                            <template v-else>
                                — {{ r.error }}
                            </template>
                        </v-alert>
                    </template>

                </v-card-text>
            </template>
        </v-card>
    </div>
</template>

<style scoped>
.captcha-status-grid {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: start;
    gap: 12px;
}

.captcha-library-status {
    display: flex;
    align-items: flex-start;
    gap: 12px;
    min-width: 0;
    padding: 12px;
    border-radius: 8px;
}

.captcha-library-status--success {
    background: rgba(var(--v-theme-success), 0.08);
}

.captcha-library-status--error {
    background: rgba(var(--v-theme-error), 0.08);
}

.captcha-status-icon {
    flex: none;
    margin-top: 1px;
}

.captcha-status-message,
.captcha-test-result {
    min-width: 0;
    line-height: 1.5;
    overflow-wrap: anywhere;
    word-break: break-word;
}

.captcha-solver-chip {
    align-self: start;
}

.captcha-test-row {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 12px;
}

@media (max-width: 720px) {
    .captcha-status-grid {
        grid-template-columns: minmax(0, 1fr);
    }
}

.worker-id-full {
    white-space: normal;
    overflow-wrap: anywhere;
    word-break: break-word;
}

.worker-id-button {
    height: auto;
    min-height: 28px;
    white-space: normal;
}
</style>
