<template>
  <component
    :is="embedded ? 'div' : BaseDialog"
    :show="show"
    :title="t('admin.scheduledTests.title')"
    width="wide"
    @close="emit('close')"
  >
    <div class="space-y-4">
      <p v-if="pelicanConfig && !groupId" class="text-xs text-gray-500">{{ t('admin.accounts.pelicanTest.scheduleHint') }}</p>
      <p v-if="groupId && !loading && !loadingKeys && !testKeys.length" class="rounded-lg bg-amber-50 p-3 text-sm text-amber-800 dark:bg-amber-900/20 dark:text-amber-300" data-testid="group-test-no-keys">{{ t('admin.scheduledTests.groupNoKeys') }}</p>
      <p v-if="groupId" class="text-xs leading-relaxed text-gray-500 dark:text-gray-400">{{ t('admin.scheduledTests.groupRetryHint') }}</p>
      <p v-if="groupId && refreshError" role="alert" class="text-xs text-amber-700 dark:text-amber-300" data-testid="group-status-reconnect">{{ t('admin.scheduledTests.groupStatusReconnect') }}</p>
      <!-- Add Plan Button -->
      <div class="flex items-center justify-between">
        <p class="text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.scheduledTests.title') }}
        </p>
        <button
          @click="showAddForm = !showAddForm"
          :disabled="disabled"
          class="btn btn-primary flex items-center gap-1.5 text-sm"
        >
          <Icon name="plus" size="sm" :stroke-width="2" />
          {{ t('admin.scheduledTests.addPlan') }}
        </button>
      </div>

      <!-- Add Plan Form -->
      <div
        v-if="showAddForm"
        class="rounded-xl border border-primary-200 bg-primary-50/50 p-4 dark:border-primary-800 dark:bg-primary-900/20"
      >
        <div class="mb-3 text-sm font-medium text-gray-700 dark:text-gray-300">
          {{ t('admin.scheduledTests.addPlan') }}
        </div>
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <div>
            <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
              {{ t('admin.scheduledTests.model') }}
            </label>
            <Input v-if="pelicanConfig" v-model="newPlan.model_id" />
            <Select v-else
              v-model="newPlan.model_id"
              :options="modelOptions"
              :placeholder="t('admin.scheduledTests.model')"
              :searchable="modelOptions.length > 5"
            />
            <template v-if="groupId">
              <label class="input-label mt-3">{{ t('admin.scheduledTests.groupTestKey') }}</label>
              <Select v-model="newPlan.api_key_id" :options="keyOptions" :searchable="true" :placeholder="t('admin.scheduledTests.groupKeyRequired')" />
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.scheduledTests.groupBillingHint') }}</p>
            </template>
          </div>
          <div>
            <label class="mb-1 flex items-center gap-1 text-xs font-medium text-gray-600 dark:text-gray-400">
              {{ t('admin.scheduledTests.cronExpression') }}
              <HelpTooltip>
                <template #trigger>
                  <span class="inline-flex h-4 w-4 cursor-help items-center justify-center rounded-full border border-gray-400/70 text-[10px] font-semibold text-gray-400 transition-colors hover:border-primary-500 hover:text-primary-600 dark:border-gray-500 dark:text-gray-500 dark:hover:border-primary-400 dark:hover:text-primary-400">
                    ?
                  </span>
                </template>
                <div class="space-y-1.5">
                  <p class="font-medium">{{ t('admin.scheduledTests.cronTooltipTitle') }}</p>
                  <p>{{ t('admin.scheduledTests.cronTooltipMeaning') }}</p>
                  <p>{{ t('admin.scheduledTests.cronTooltipExampleEvery30Min') }}</p>
                  <p>{{ t('admin.scheduledTests.cronTooltipExampleHourly') }}</p>
                  <p>{{ t('admin.scheduledTests.cronTooltipExampleDaily') }}</p>
                  <p>{{ t('admin.scheduledTests.cronTooltipExampleWeekly') }}</p>
                  <p>{{ t('admin.scheduledTests.cronTooltipRange') }}</p>
                </div>
              </HelpTooltip>
            </label>
            <Input
              v-model="newPlan.cron_expression"
              :placeholder="'*/30 * * * *'"
              :hint="t('admin.scheduledTests.cronHelp')"
            />
          </div>
          <div>
            <label class="mb-1 flex items-center gap-1 text-xs font-medium text-gray-600 dark:text-gray-400">
              {{ t('admin.scheduledTests.maxResults') }}
              <HelpTooltip>
                <template #trigger>
                  <span class="inline-flex h-4 w-4 cursor-help items-center justify-center rounded-full border border-gray-400/70 text-[10px] font-semibold text-gray-400 transition-colors hover:border-primary-500 hover:text-primary-600 dark:border-gray-500 dark:text-gray-500 dark:hover:border-primary-400 dark:hover:text-primary-400">
                    ?
                  </span>
                </template>
                <div class="space-y-1.5">
                  <p class="font-medium">{{ t('admin.scheduledTests.maxResultsTooltipTitle') }}</p>
                  <p>{{ t('admin.scheduledTests.maxResultsTooltipMeaning') }}</p>
                  <p>{{ t('admin.scheduledTests.maxResultsTooltipBody') }}</p>
                  <p>{{ t('admin.scheduledTests.maxResultsTooltipExample') }}</p>
                  <p>{{ t('admin.scheduledTests.maxResultsTooltipRange') }}</p>
                </div>
              </HelpTooltip>
            </label>
            <Input
              v-model="newPlan.max_results"
              type="number"
              placeholder="100"
            />
          </div>
          <div class="flex items-end">
            <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
              <Toggle v-model="newPlan.enabled" />
              {{ t('admin.scheduledTests.enabled') }}
            </label>
          </div>
          <div v-if="!groupId" class="flex items-end">
            <div>
              <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
                <Toggle v-model="newPlan.auto_recover" />
                {{ t('admin.scheduledTests.autoRecover') }}
              </label>
              <p class="mt-0.5 text-xs text-gray-400 dark:text-gray-500">
                {{ t('admin.scheduledTests.autoRecoverHelp') }}
              </p>
            </div>
          </div>
        </div>
        <PelicanTestFields v-if="pelicanConfig" v-model="newPelican" :drawing-only="Boolean(groupId)" class="mt-3" />
        <div class="mt-3 flex justify-end gap-2">
          <button
            @click="showAddForm = false; resetNewPlan()"
            class="rounded-lg bg-gray-100 px-3 py-1.5 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-200 dark:bg-dark-600 dark:text-gray-300 dark:hover:bg-dark-500"
          >
            {{ t('common.cancel') }}
          </button>
          <button
            @click="handleCreate"
            :disabled="disabled || !newPlan.model_id || !newPlan.cron_expression || creating"
            class="flex items-center gap-1.5 rounded-lg bg-primary-500 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-primary-600 disabled:cursor-not-allowed disabled:opacity-50"
          >
            <Icon v-if="creating" name="refresh" size="sm" class="animate-spin" :stroke-width="2" />
            {{ t('common.save') }}
          </button>
        </div>
      </div>

      <!-- Loading State -->
      <div v-if="loading" class="flex items-center justify-center py-8">
        <Icon name="refresh" size="md" class="animate-spin text-gray-400" :stroke-width="2" />
        <span class="ml-2 text-sm text-gray-500">{{ t('common.loading') }}...</span>
      </div>

      <!-- Empty State -->
      <div
        v-else-if="plans.length === 0"
        class="rounded-xl border border-dashed border-gray-300 py-10 text-center dark:border-dark-600"
      >
        <Icon name="calendar" size="lg" class="mx-auto mb-2 text-gray-400" :stroke-width="1.5" />
        <p class="text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.scheduledTests.noPlans') }}
        </p>
      </div>

      <!-- Plans List -->
      <div v-else class="space-y-3">
        <div
          v-for="plan in plans"
          :key="plan.id"
          class="rounded-xl border border-gray-200 bg-white transition-all dark:border-dark-600 dark:bg-dark-800"
        >
          <!-- Plan Header -->
          <div
            class="flex cursor-pointer items-center justify-between px-4 py-3"
            @click="toggleExpand(plan.id)"
          >
            <div class="flex flex-1 items-center gap-4">
              <!-- Model -->
              <div class="min-w-0">
                <div class="text-sm font-medium text-gray-900 dark:text-gray-100">
                  {{ plan.model_id }}
                </div>
                <div class="mt-0.5 font-mono text-xs text-gray-500 dark:text-gray-400">
                  {{ plan.cron_expression }}
                </div>
              </div>

              <!-- Enabled Toggle -->
              <div class="flex items-center gap-1.5" @click.stop>
                <Toggle
                  :model-value="plan.enabled"
                  @update:model-value="(val: boolean) => handleToggleEnabled(plan, val)"
                />
                <span class="text-xs text-gray-500 dark:text-gray-400">
                  {{ plan.enabled ? t('admin.scheduledTests.enabled') : '' }}
                </span>
              </div>

              <!-- Auto Recover Badge -->
              <span
                v-if="plan.auto_recover"
                class="inline-flex items-center rounded-full bg-emerald-100 px-2 py-0.5 text-xs font-medium text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-400"
              >
                {{ t('admin.scheduledTests.autoRecover') }}
              </span>
            </div>

            <div class="flex items-center gap-3">
              <!-- Last Run -->
              <div v-if="plan.last_run_at" class="hidden text-right text-xs text-gray-500 dark:text-gray-400 sm:block">
                <div>{{ t('admin.scheduledTests.lastRun') }}</div>
                <div>{{ formatDateTime(plan.last_run_at) }}</div>
              </div>

              <!-- Next Run -->
              <div v-if="plan.enabled && plan.next_run_at" class="hidden text-right text-xs text-gray-500 dark:text-gray-400 sm:block">
                <div>{{ t('admin.scheduledTests.nextRun') }}</div>
                <div>{{ formatDateTime(plan.next_run_at) }}</div>
              </div>

              <!-- Actions -->
              <div class="flex items-center gap-1" @click.stop>
                <button v-if="groupId" type="button" data-testid="trigger-group-test" class="rounded-lg p-1.5 text-gray-500 hover:bg-primary-50 hover:text-primary-600 disabled:opacity-40 dark:hover:bg-primary-900/20" :disabled="disabled || !plan.enabled || groupBusy || refreshError" :title="t('admin.scheduledTests.triggerGroup')" @click="triggerPlan(plan)">
                  <Icon :name="triggering === plan.id ? 'refresh' : 'play'" size="sm" :class="triggering === plan.id ? 'animate-spin' : ''" />
                </button>
                <button
                  v-if="isGroupPlanRunning(plan)"
                  type="button"
                  data-testid="cancel-group-test"
                  class="inline-flex items-center gap-1.5 rounded-lg px-2 py-1.5 text-xs font-medium text-red-600 transition-colors hover:bg-red-50 disabled:cursor-wait disabled:opacity-50 dark:text-red-400 dark:hover:bg-red-900/20"
                  :disabled="disabled || executionStatus(plan) === 'cancelling'"
                  :title="t('admin.scheduledTests.cancelGroupHint')"
                  @click="cancelPlan(plan)"
                >
                  <Icon v-if="executionStatus(plan) === 'cancelling'" name="refresh" size="sm" class="animate-spin" />
                  {{ t(executionStatus(plan) === 'cancelling' ? 'admin.scheduledTests.groupStatusCancelling' : 'admin.scheduledTests.cancelGroup') }}
                </button>
                <button
                  @click="startEdit(plan)"
                  class="rounded-lg p-1.5 text-gray-400 transition-colors hover:bg-blue-50 hover:text-blue-500 dark:hover:bg-blue-900/20"
                  :title="t('admin.scheduledTests.editPlan')"
                >
                  <Icon name="edit" size="sm" :stroke-width="2" />
                </button>
                <button
                  @click="confirmDeletePlan(plan)"
                  class="rounded-lg p-1.5 text-gray-400 transition-colors hover:bg-red-50 hover:text-red-500 dark:hover:bg-red-900/20"
                  :title="t('admin.scheduledTests.deletePlan')"
                >
                  <Icon name="trash" size="sm" :stroke-width="2" />
                </button>
              </div>

              <!-- Expand indicator -->
              <Icon
                name="chevronDown"
                size="sm"
                :class="[
                  'text-gray-400 transition-transform duration-200',
                  expandedPlanId === plan.id ? 'rotate-180' : ''
                ]"
              />
            </div>
          </div>

          <div v-if="groupId" class="space-y-2 border-t border-gray-100 bg-gray-50/60 px-4 py-3 dark:border-dark-700 dark:bg-dark-900/30" role="status" aria-live="polite" :data-testid="'group-test-status-' + plan.id">
            <div class="flex flex-wrap items-center gap-2 text-xs">
              <span class="inline-flex items-center gap-1.5 rounded-full px-2 py-1 font-medium" :class="executionStatusClass(plan)">
                <Icon v-if="['starting', 'running', 'retrying', 'cancelling'].includes(executionStatus(plan))" name="refresh" size="sm" class="animate-spin" />
                {{ executionStatusLabel(plan) }}
              </span>
              <span v-if="plan.execution" class="tabular-nums text-gray-500 dark:text-gray-400">{{ t('admin.scheduledTests.groupElapsed', { seconds: executionElapsed(plan) }) }}</span>
              <span v-if="executionStatus(plan) === 'retrying' && plan.execution?.retry_at" class="tabular-nums text-amber-700 dark:text-amber-300">{{ t('admin.scheduledTests.groupRetryCountdown', { seconds: Math.max(0, Math.ceil((Date.parse(plan.execution.retry_at) - clockNow) / 1000)) }) }}</span>
            </div>
            <p v-if="plan.execution" class="text-xs tabular-nums text-gray-600 dark:text-gray-300">{{ t('admin.scheduledTests.groupProgress', { attempt: plan.execution.attempt, max: plan.execution.max_attempts, completed: plan.execution.completed, total: plan.execution.total, succeeded: plan.execution.succeeded, failed: plan.execution.failed }) }}</p>
            <p v-if="plan.execution?.last_error" class="break-words text-xs text-red-600 dark:text-red-300">{{ t('admin.scheduledTests.groupLastError', { message: plan.execution.last_error }) }}</p>
          </div>

          <!-- Edit Form -->
          <div
            v-if="editingPlanId === plan.id"
            class="border-t border-blue-100 bg-blue-50/50 px-4 py-3 dark:border-blue-900 dark:bg-blue-900/10"
            @click.stop
          >
            <div class="mb-2 text-xs font-medium text-gray-600 dark:text-gray-400">
              {{ t('admin.scheduledTests.editPlan') }}
            </div>
            <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <div>
                <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                  {{ t('admin.scheduledTests.model') }}
                </label>
                <Input v-if="pelicanConfig" v-model="editForm.model_id" />
                <Select v-else
                  v-model="editForm.model_id"
                  :options="modelOptions"
                  :placeholder="t('admin.scheduledTests.model')"
                  :searchable="modelOptions.length > 5"
                />
                <template v-if="groupId">
                  <label class="input-label mt-3">{{ t('admin.scheduledTests.groupTestKey') }}</label>
                  <Select v-model="editForm.api_key_id" :options="keyOptions" :searchable="true" :placeholder="t('admin.scheduledTests.groupKeyRequired')" />
                  <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('admin.scheduledTests.groupBillingHint') }}</p>
                </template>
              </div>
              <div>
                <label class="mb-1 flex items-center gap-1 text-xs font-medium text-gray-600 dark:text-gray-400">
                  {{ t('admin.scheduledTests.cronExpression') }}
                  <HelpTooltip>
                    <template #trigger>
                      <span class="inline-flex h-4 w-4 cursor-help items-center justify-center rounded-full border border-gray-400/70 text-[10px] font-semibold text-gray-400 transition-colors hover:border-primary-500 hover:text-primary-600 dark:border-gray-500 dark:text-gray-500 dark:hover:border-primary-400 dark:hover:text-primary-400">
                        ?
                      </span>
                    </template>
                    <div class="space-y-1.5">
                      <p class="font-medium">{{ t('admin.scheduledTests.cronTooltipTitle') }}</p>
                      <p>{{ t('admin.scheduledTests.cronTooltipMeaning') }}</p>
                      <p>{{ t('admin.scheduledTests.cronTooltipExampleEvery30Min') }}</p>
                      <p>{{ t('admin.scheduledTests.cronTooltipExampleHourly') }}</p>
                      <p>{{ t('admin.scheduledTests.cronTooltipExampleDaily') }}</p>
                      <p>{{ t('admin.scheduledTests.cronTooltipExampleWeekly') }}</p>
                      <p>{{ t('admin.scheduledTests.cronTooltipRange') }}</p>
                    </div>
                  </HelpTooltip>
                </label>
                <Input
                  v-model="editForm.cron_expression"
                  :placeholder="'*/30 * * * *'"
                  :hint="t('admin.scheduledTests.cronHelp')"
                />
              </div>
              <div>
                <label class="mb-1 flex items-center gap-1 text-xs font-medium text-gray-600 dark:text-gray-400">
                  {{ t('admin.scheduledTests.maxResults') }}
                  <HelpTooltip>
                    <template #trigger>
                      <span class="inline-flex h-4 w-4 cursor-help items-center justify-center rounded-full border border-gray-400/70 text-[10px] font-semibold text-gray-400 transition-colors hover:border-primary-500 hover:text-primary-600 dark:border-gray-500 dark:text-gray-500 dark:hover:border-primary-400 dark:hover:text-primary-400">
                        ?
                      </span>
                    </template>
                    <div class="space-y-1.5">
                      <p class="font-medium">{{ t('admin.scheduledTests.maxResultsTooltipTitle') }}</p>
                      <p>{{ t('admin.scheduledTests.maxResultsTooltipMeaning') }}</p>
                      <p>{{ t('admin.scheduledTests.maxResultsTooltipBody') }}</p>
                      <p>{{ t('admin.scheduledTests.maxResultsTooltipExample') }}</p>
                      <p>{{ t('admin.scheduledTests.maxResultsTooltipRange') }}</p>
                    </div>
                  </HelpTooltip>
                </label>
                <Input
                  v-model="editForm.max_results"
                  type="number"
                  placeholder="100"
                />
              </div>
              <div class="flex items-end">
                <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
                  <Toggle v-model="editForm.enabled" />
                  {{ t('admin.scheduledTests.enabled') }}
                </label>
              </div>
              <div v-if="!groupId" class="flex items-end">
                <div>
                  <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
                    <Toggle v-model="editForm.auto_recover" />
                    {{ t('admin.scheduledTests.autoRecover') }}
                  </label>
                  <p class="mt-0.5 text-xs text-gray-400 dark:text-gray-500">
                    {{ t('admin.scheduledTests.autoRecoverHelp') }}
                  </p>
                </div>
              </div>
            </div>
            <PelicanTestFields v-if="pelicanConfig" v-model="editPelican" :drawing-only="Boolean(groupId)" class="mt-3" />
            <div class="mt-3 flex justify-end gap-2">
              <button
                @click="cancelEdit"
                class="rounded-lg bg-gray-100 px-3 py-1.5 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-200 dark:bg-dark-600 dark:text-gray-300 dark:hover:bg-dark-500"
              >
                {{ t('common.cancel') }}
              </button>
              <button
                @click="handleEdit"
                :disabled="disabled || !editForm.model_id || !editForm.cron_expression || updating"
                class="flex items-center gap-1.5 rounded-lg bg-primary-500 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-primary-600 disabled:cursor-not-allowed disabled:opacity-50"
              >
                <Icon v-if="updating" name="refresh" size="sm" class="animate-spin" :stroke-width="2" />
                {{ t('common.save') }}
              </button>
            </div>
          </div>

          <!-- Expanded Results Section -->
          <div
            v-if="expandedPlanId === plan.id"
            class="border-t border-gray-100 px-4 py-3 dark:border-dark-700"
          >
            <div class="mb-2 text-xs font-medium text-gray-600 dark:text-gray-400">
              {{ t('admin.scheduledTests.results') }}
            </div>

            <!-- Results Loading -->
            <div v-if="loadingResults" class="flex items-center justify-center py-4">
              <Icon name="refresh" size="sm" class="animate-spin text-gray-400" :stroke-width="2" />
              <span class="ml-2 text-xs text-gray-500">{{ t('common.loading') }}...</span>
            </div>

            <!-- No Results -->
            <div
              v-else-if="results.length === 0"
              class="py-4 text-center text-xs text-gray-500 dark:text-gray-400"
            >
              {{ t('admin.scheduledTests.noResults') }}
            </div>

            <!-- Results List -->
            <div v-else class="max-h-64 space-y-2 overflow-y-auto">
              <div
                v-for="result in results"
                :key="result.id"
                class="rounded-lg border border-gray-100 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-900"
              >
                <div class="flex items-center justify-between">
                  <div class="flex items-center gap-2">
                    <!-- Status Badge -->
                    <span
                      :class="[
                        'inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium',
                        result.status === 'success'
                          ? 'bg-green-100 text-green-700 dark:bg-green-500/20 dark:text-green-400'
                          : result.status === 'running'
                            ? 'bg-blue-100 text-blue-700 dark:bg-blue-500/20 dark:text-blue-400'
                            : 'bg-red-100 text-red-700 dark:bg-red-500/20 dark:text-red-400'
                      ]"
                    >
                      {{
                        result.status === 'success'
                          ? t('admin.scheduledTests.success')
                          : result.status === 'running'
                            ? t('admin.scheduledTests.running')
                            : t('admin.scheduledTests.failed')
                      }}
                    </span>

                    <!-- Latency -->
                    <span v-if="result.latency_ms > 0" class="text-xs text-gray-500 dark:text-gray-400">
                      {{ pelicanConfig ? `${t('admin.accounts.pelicanTest.duration')} ${(result.latency_ms / 1000).toFixed(1)} s` : `${result.latency_ms}ms` }}
                    </span>
                  </div>

                  <!-- Started At -->
                  <span class="text-xs text-gray-400">
                    <span v-if="pelicanConfig">{{ t('admin.accounts.pelicanTest.generatedAt') }}：</span>{{ formatDateTime(result.started_at) }}
                  </span>
                </div>

                <div v-if="pelicanConfig" class="mt-2 text-xs text-gray-500 dark:text-gray-400">
                  {{ t('admin.accounts.pelicanTest.sourceScheduled') }} · {{ result.pelican_config?.model_id || '—' }} / {{ result.pelican_config?.reasoning_effort || '—' }}
                </div>
                <button v-if="pelicanConfig" type="button" class="mt-2 text-xs text-primary-600" :disabled="disabled" @click="previewResult(result)">
                  {{ t('admin.accounts.pelicanTest.preview') }}
                </button>
                <!-- Response / Error (collapsible) -->
                <div v-if="result.error_message" class="mt-2">
                  <div
                    class="cursor-pointer text-xs font-medium text-red-600 dark:text-red-400"
                    @click="toggleResultDetail(result.id)"
                  >
                    {{ t('admin.scheduledTests.errorMessage') }}
                    <Icon
                      name="chevronDown"
                      size="sm"
                      :class="[
                        'inline transition-transform duration-200',
                        expandedResultIds.has(result.id) ? 'rotate-180' : ''
                      ]"
                    />
                  </div>
                  <pre
                    v-if="expandedResultIds.has(result.id)"
                    class="mt-1 max-h-32 overflow-auto whitespace-pre-wrap rounded bg-red-50 p-2 text-xs text-red-700 dark:bg-red-900/20 dark:text-red-300"
                  >{{ result.error_message }}</pre>
                </div>
                <div v-else-if="result.response_text" class="mt-2">
                  <div
                    class="cursor-pointer text-xs font-medium text-gray-600 dark:text-gray-400"
                    @click="toggleResultDetail(result.id)"
                  >
                    {{ t('admin.scheduledTests.responseText') }}
                    <Icon
                      name="chevronDown"
                      size="sm"
                      :class="[
                        'inline transition-transform duration-200',
                        expandedResultIds.has(result.id) ? 'rotate-180' : ''
                      ]"
                    />
                  </div>
                  <pre
                    v-if="expandedResultIds.has(result.id)"
                    class="mt-1 max-h-32 overflow-auto whitespace-pre-wrap rounded bg-gray-100 p-2 text-xs text-gray-700 dark:bg-dark-800 dark:text-gray-300"
                  >{{ result.response_text }}</pre>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Delete Confirmation -->
    <ConfirmDialog
      :show="showDeleteConfirm"
      :title="t('admin.scheduledTests.deletePlan')"
      :message="t('admin.scheduledTests.confirmDelete')"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="handleDelete"
      @cancel="showDeleteConfirm = false"
    />
  </component>
</template>

<script setup lang="ts">
import { computed, ref, reactive, watch, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import Input from '@/components/common/Input.vue'
import Toggle from '@/components/common/Toggle.vue'
import { Icon } from '@/components/icons'
import type { GroupTestKey } from '@/api/admin/scheduledTests'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from '@/utils/format'
import PelicanTestFields from './PelicanTestFields.vue'
import type { PelicanTestConfig, ScheduledTestPlan, ScheduledTestResult } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()
const testKeys = ref<GroupTestKey[]>([])
const loadingKeys = ref(false)
const keyOptions = computed(() => testKeys.value.map(key => ({ value: key.id, label: key.name + " · " + key.user_email + " (#" + key.id + ")" })))
const triggering = ref<number | null>(null)

const props = defineProps<{
  show: boolean
  accountId: number | null
  groupId?: number
  modelOptions: SelectOption[]
  embedded?: boolean
  pelicanConfig?: PelicanTestConfig
  defaultModel?: string
  disabled?: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'preview', result: ScheduledTestResult): void
  (e: 'history', results: ScheduledTestResult[]): void
}>()

const targetId = computed(() => props.groupId || props.accountId)

const configDefaults = (): PelicanTestConfig => ({ prompt: '', reasoning_effort: 'medium', parallel_count: 1, ...props.pelicanConfig })
const newPelican = ref(configDefaults())
const editPelican = ref(configDefaults())

let alive = true
let revision = 0
let triggerSequence = 0
let cancelSequence = 0
const cancelling = ref<number | null>(null)

// State
const loading = ref(false)
const refreshError = ref(false)
const clockNow = ref(Date.now())
const creating = ref(false)
const loadingResults = ref(false)
const plans = ref<ScheduledTestPlan[]>([])
const results = ref<ScheduledTestResult[]>([])
const expandedPlanId = ref<number | null>(null)
const expandedResultIds = reactive(new Set<number>())
const showAddForm = ref(false)
const showDeleteConfirm = ref(false)
const deletingPlan = ref<ScheduledTestPlan | null>(null)
const editingPlanId = ref<number | null>(null)
const updating = ref(false)
const editForm = reactive({
  model_id: props.defaultModel || '',
  cron_expression: '*/30 * * * *',
  max_results: '100' as string,
  enabled: true,
  auto_recover: false,
  api_key_id: 0
})

const newPlan = reactive({
  model_id: props.defaultModel || '',
  cron_expression: '*/30 * * * *',
  max_results: '100' as string,
  enabled: true,
  auto_recover: false,
  api_key_id: 0
})

const resetNewPlan = () => {
  newPlan.model_id = props.defaultModel || ''
  newPelican.value = configDefaults()
  newPlan.cron_expression = '*/30 * * * *'
  newPlan.max_results = '100'
  newPlan.enabled = true
  newPlan.auto_recover = false
  newPlan.api_key_id = 0
}

const loadPlans = async (silent = false) => {
  if (!targetId.value) return
  const scopeId = targetId.value
  const version = revision
  if (!silent) loading.value = true
  try {
    const data = props.groupId
      ? await adminAPI.scheduledTests.listByGroup(props.groupId)
      : await adminAPI.scheduledTests.listByAccount(scopeId)
    if (alive && props.show && targetId.value === scopeId && revision === version) {
      refreshError.value = false
      plans.value = data.filter((plan) => Boolean(plan.pelican_config) === Boolean(props.pelicanConfig))
      if (!silent && props.pelicanConfig && plans.value.length > 0 && expandedPlanId.value === null) {
        await expandPlan(plans.value[0].id)
      }
    }
  } catch (error: any) {
    if (alive && props.show && targetId.value === scopeId && revision === version) {
      if (silent) refreshError.value = true
      else appStore.showError(error?.message || 'Failed to load plans')
    }
  } finally {
    if (!silent && alive && revision === version) loading.value = false
  }
}

const handleCreate = async () => {
  if (!targetId.value || !newPlan.model_id || !newPlan.cron_expression) return
  if (props.groupId && !newPlan.api_key_id) { appStore.showError(t("admin.scheduledTests.groupKeyRequired")); return }
  if (props.disabled || creating.value) return
  revision++
  creating.value = true
  try {
    const maxResults = Number(newPlan.max_results) || 100
    await adminAPI.scheduledTests.create({
      ...(props.groupId ? { group_id: props.groupId, api_key_id: Number(newPlan.api_key_id) } : { account_id: props.accountId! }),
      model_id: newPlan.model_id,
      cron_expression: newPlan.cron_expression,
      enabled: newPlan.enabled,
      max_results: maxResults,
      auto_recover: props.groupId ? false : newPlan.auto_recover,
      ...(props.pelicanConfig ? { pelican_config: newPelican.value } : {})
    })
    appStore.showSuccess(t('admin.scheduledTests.createSuccess'))
    showAddForm.value = false
    resetNewPlan()
    await loadPlans()
  } catch (error: any) {
    appStore.showError(error?.message || 'Failed to create plan')
  } finally {
    creating.value = false
  }
}

const handleToggleEnabled = async (plan: ScheduledTestPlan, enabled: boolean) => {
  if (props.disabled) return
  revision++
  try {
    const updated = await adminAPI.scheduledTests.update(plan.id, { enabled })
    const index = plans.value.findIndex((p) => p.id === plan.id)
    if (index !== -1) {
      plans.value[index] = updated
    }
    appStore.showSuccess(t('admin.scheduledTests.updateSuccess'))
  } catch (error: any) {
    appStore.showError(error?.message || 'Failed to update plan')
  }
}

const startEdit = (plan: ScheduledTestPlan) => {
  editingPlanId.value = plan.id
  editForm.model_id = plan.model_id
  editForm.cron_expression = plan.cron_expression
  editForm.max_results = String(plan.max_results)
  editForm.enabled = plan.enabled
  editForm.auto_recover = plan.auto_recover
  editForm.api_key_id = plan.api_key_id || 0
  if (plan.pelican_config) editPelican.value = { ...plan.pelican_config }
}

const cancelEdit = () => {
  editingPlanId.value = null
}

const handleEdit = async () => {
  if (!editingPlanId.value || !editForm.model_id || !editForm.cron_expression) return
  if (props.groupId && editForm.enabled && !editForm.api_key_id) { appStore.showError(t("admin.scheduledTests.groupKeyRequired")); return }
  if (props.disabled || updating.value) return
  revision++
  updating.value = true
  try {
    const updated = await adminAPI.scheduledTests.update(editingPlanId.value, {
      model_id: editForm.model_id,
      cron_expression: editForm.cron_expression,
      max_results: Number(editForm.max_results) || 100,
      enabled: editForm.enabled,
      auto_recover: props.groupId ? false : editForm.auto_recover,
      ...(props.groupId ? { api_key_id: Number(editForm.api_key_id) } : {}),
      ...(props.pelicanConfig ? { pelican_config: editPelican.value } : {})
    })
    const index = plans.value.findIndex((p) => p.id === editingPlanId.value)
    if (index !== -1) {
      plans.value[index] = updated
    }
    appStore.showSuccess(t('admin.scheduledTests.updateSuccess'))
    editingPlanId.value = null
  } catch (error: any) {
    appStore.showError(error?.message || 'Failed to update plan')
  } finally {
    updating.value = false
  }
}

const isGroupPlanRunning = (plan: ScheduledTestPlan) => Boolean(props.groupId && plan.running_until && Date.parse(plan.running_until) > clockNow.value && !['success', 'failed', 'interrupted'].includes(plan.execution?.status || ''))
const executionStatus = (plan: ScheduledTestPlan) => {
  if (cancelling.value === plan.id) return 'cancelling'
  if (triggering.value === plan.id) return 'starting'
  const leased = Boolean(plan.running_until && Date.parse(plan.running_until) > clockNow.value)
  if (plan.execution?.status === 'running' || plan.execution?.status === 'retrying' || plan.execution?.status === 'cancelling') return leased ? plan.execution.status : 'interrupted'
  if (leased) return 'running'
  return plan.execution?.status || (plan.enabled ? 'ready' : 'paused')
}
const groupBusy = computed(() => triggering.value !== null || cancelling.value !== null || plans.value.some(plan => Boolean(plan.running_until && Date.parse(plan.running_until) > clockNow.value)))
const executionStatusLabel = (plan: ScheduledTestPlan) => {
  const status = executionStatus(plan)
  if (status === 'running' && plan.execution?.phase) {
    const phases = {
      waiting: 'admin.scheduledTests.groupPhaseWaiting',
      receiving: 'admin.scheduledTests.groupPhaseReceiving',
      thinking: 'admin.scheduledTests.groupPhaseThinking',
      generating: 'admin.scheduledTests.groupPhaseGenerating',
      saving: 'admin.scheduledTests.groupPhaseSaving',
    }
    return t(phases[plan.execution.phase])
  }
  const labels = {
    starting: 'admin.scheduledTests.groupStatusStarting',
    running: 'admin.scheduledTests.groupStatusRunning',
    retrying: 'admin.scheduledTests.groupStatusRetrying',
    cancelling: 'admin.scheduledTests.groupStatusCancelling',
    success: 'admin.scheduledTests.groupStatusSuccess',
    failed: 'admin.scheduledTests.groupStatusFailed',
    interrupted: 'admin.scheduledTests.groupStatusInterrupted',
    ready: 'admin.scheduledTests.groupStatusReady',
    paused: 'admin.scheduledTests.groupStatusPaused',
  }
  return t(labels[status])
}
const executionStatusClass = (plan: ScheduledTestPlan) => {
  const status = executionStatus(plan)
  if (status === 'success') return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300'
  if (status === 'failed' || status === 'interrupted') return 'bg-red-50 text-red-700 dark:bg-red-900/20 dark:text-red-300'
  if (status === 'retrying' || status === 'cancelling') return 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-300'
  if (status === 'starting' || status === 'running') return 'bg-primary-50 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300'
  return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
}
const executionElapsed = (plan: ScheduledTestPlan) => {
  const state = plan.execution
  if (!state) return 0
  const end = state.finished_at ? Date.parse(state.finished_at) : clockNow.value
  return Math.max(0, Math.floor((end - Date.parse(state.started_at)) / 1000))
}

const triggerPlan = async (plan: ScheduledTestPlan) => {
  if (!props.groupId || groupBusy.value || !plan.enabled || props.disabled) return
  const scopeId = targetId.value
  const version = ++revision
  const sequence = ++triggerSequence
  triggering.value = plan.id
  try {
    await adminAPI.scheduledTests.triggerGroupPlan(plan.id)
    if (!alive || !props.show || targetId.value !== scopeId || revision !== version) return
    appStore.showSuccess(t('admin.scheduledTests.groupStarted'))
    await loadPlans(true)
    if (alive && props.show && targetId.value === scopeId && revision === version) await expandPlan(plan.id)
  } catch (error: any) {
    if (alive && props.show && targetId.value === scopeId && revision === version) appStore.showError(error?.message || t('admin.scheduledTests.groupTriggerError'))
  } finally { if (alive && triggerSequence === sequence) triggering.value = null }
}

const cancelPlan = async (plan: ScheduledTestPlan) => {
  if (!isGroupPlanRunning(plan) || !plan.running_until || props.disabled || executionStatus(plan) === 'cancelling') return
  const scopeId = targetId.value
  const version = ++revision
  const sequence = ++cancelSequence
  cancelling.value = plan.id
  try {
    await adminAPI.scheduledTests.cancelGroupPlan(plan.id, plan.running_until)
    if (!alive || !props.show || targetId.value !== scopeId || revision !== version) return
    appStore.showSuccess(t('admin.scheduledTests.groupCancelRequested'))
    await loadPlans(true)
  } catch (error: any) {
    if (alive && props.show && targetId.value === scopeId && revision === version) appStore.showError(error?.message || t('admin.scheduledTests.groupCancelError'))
  } finally { if (alive && cancelSequence === sequence) cancelling.value = null }
}

const confirmDeletePlan = (plan: ScheduledTestPlan) => {
  deletingPlan.value = plan
  showDeleteConfirm.value = true
}

const handleDelete = async () => {
  if (!deletingPlan.value || props.disabled) return
  revision++
  try {
    await adminAPI.scheduledTests.delete(deletingPlan.value.id)
    appStore.showSuccess(t('admin.scheduledTests.deleteSuccess'))
    plans.value = plans.value.filter((p) => p.id !== deletingPlan.value!.id)
    if (expandedPlanId.value === deletingPlan.value.id) {
      expandedPlanId.value = null
      results.value = []
    }
  } catch (error: any) {
    appStore.showError(error?.message || 'Failed to delete plan')
  } finally {
    showDeleteConfirm.value = false
    deletingPlan.value = null
  }
}

const expandPlan = async (planId: number) => {
  const scopeId = targetId.value
  expandedPlanId.value = planId
  expandedResultIds.clear()
  loadingResults.value = true
  try {
    const data = await adminAPI.scheduledTests.listResults(planId, 20, !props.pelicanConfig)
    if (alive && props.show && targetId.value === scopeId && expandedPlanId.value === planId) { results.value = data; emit('history', data) }
  } catch (error: any) {
    appStore.showError(error?.message || 'Failed to load results')
    results.value = []
  } finally {
    loadingResults.value = false
  }
}

const toggleExpand = async (planId: number) => {
  if (expandedPlanId.value === planId) {
    expandedPlanId.value = null
    results.value = []
    expandedResultIds.clear()
    return
  }
  await expandPlan(planId)
}

const toggleResultDetail = (resultId: number) => {
  if (expandedResultIds.has(resultId)) {
    expandedResultIds.delete(resultId)
  } else {
    expandedResultIds.add(resultId)
  }
}
const previewResult = async (result: ScheduledTestResult) => {
  if (props.disabled) return
  const scopeId = targetId.value
  try {
    const full = await adminAPI.scheduledTests.getResult(result.plan_id, result.id)
    if (alive && props.show && targetId.value === scopeId) emit('preview', full)
  } catch (error: any) {
    appStore.showError(error?.message || 'Failed to load result')
  }
}

watch(() => [props.show, props.accountId, props.groupId] as const, async ([visible]) => {
  revision++
  triggerSequence++
  cancelSequence++
  cancelling.value = null
  triggering.value = null
  refreshError.value = false
  loading.value = false
  plans.value = []
  results.value = []
  emit('history', [])
  expandedPlanId.value = null
  expandedResultIds.clear()
  showAddForm.value = false
  showDeleteConfirm.value = false
  editingPlanId.value = null
  resetNewPlan()
  testKeys.value = []
  loadingKeys.value = false
  if (visible && targetId.value) {
    const version = revision
    if (props.groupId) {
      loadingKeys.value = true
      try {
        const keys = await adminAPI.scheduledTests.listGroupTestKeys(props.groupId)
        if (alive && props.show && revision === version) testKeys.value = keys
      } catch (error: any) { appStore.showError(error?.message || t('admin.scheduledTests.groupKeysError')) }
      finally { if (alive && revision === version) loadingKeys.value = false }
    }
    if (alive && props.show && revision === version) await loadPlans()
  }
}, { immediate: true })

// Poll group progress without hiding cards or overlapping requests. Account history keeps its cadence.
let refreshInFlight = false
let lastRefreshAt = Date.now()
const refreshTimer = setInterval(async () => {
  clockNow.value = Date.now()
  if (!props.show || !props.pelicanConfig || loading.value || loadingKeys.value || creating.value || updating.value || triggering.value !== null || refreshInFlight) return
  const interval = props.groupId ? 1000 : 15000
  if (clockNow.value - lastRefreshAt < interval) return
  lastRefreshAt = clockNow.value
  refreshInFlight = true
  const scopeId = targetId.value
  const version = revision
  try {
    await loadPlans(true)
    const id = expandedPlanId.value
    if (!alive || !props.show || revision !== version || targetId.value !== scopeId || !id) return
    const data = await adminAPI.scheduledTests.listResults(id, 20, false)
    if (alive && props.show && revision === version && targetId.value === scopeId && expandedPlanId.value === id) {
      results.value = data
      emit('history', data)
    }
  } catch { /* Manual expansion still surfaces errors. */ }
  finally { refreshInFlight = false }
}, 1000)
onBeforeUnmount(() => { alive = false; clearInterval(refreshTimer) })
</script>
