export default {
  accountModelMismatch: {
    label: 'Model degradation',
    legacyLabel: 'Historical model record',
    legacyExplanation: 'This historical record is not gpt-6-astra → gpt-5.6-luna and no longer counts as degradation. Existing pauses remain unchanged; the account is never enabled automatically.',
    legacyRestoreHint: 'After review, explicitly resume this account and clear its historical model marker. Manually disabled IPs, authentication errors and cooldowns remain in place.',
    viewDetails: 'View model mismatch evidence and scheduling state',
    title: 'Model mismatch',
    paused: 'Scheduling paused',
    observed: 'Recorded without automatic pause',
    observedExplanation: 'Automatic model mismatch quarantine was disabled when this was detected. The mismatch was recorded without changing scheduling. Manual pauses, rate limits and other protections still apply.',
    explanation: 'The request sent gpt-6-astra and the upstream response declared gpt-5.6-luna. New requests are paused for this account and all of its fixed IP channels. This flag uses the model field in the upstream response.',
    expected: 'Model sent upstream',
    actual: 'Upstream response model',
    detectedAt: 'Detected at',
    requestId: 'Request ID',
    restoreHint: 'After verifying upstream recovery, confirm to resume scheduling. Manually paused IPs, other errors and cooldowns are preserved. Another gpt-6-astra → gpt-5.6-luna response will pause the account again while automatic quarantine is enabled.',
    confirmRestore: 'Confirm resume scheduling',
    restoreFailed: 'Unable to resume scheduling. Please try again.',
    channelsPaused: 'Model mismatch: all fixed IP channels are paused. Click the model mismatch badge in the account list to review and confirm resuming.',
    settings: {
      title: 'Automatically pause on model mismatch',
      description: 'Pause the account and all fixed IP channels only when the request sends gpt-6-astra and the upstream declares gpt-5.6-luna. Changes save immediately. Enabled by default.',
      existingHint: 'When disabled, new detections keep a red model mismatch record without automatically pausing. Existing quarantines require individual manual recovery from the account list. Manual pauses and other limits are preserved.',
      stateHint: 'This switch is independent of STATE. STATE model validation, ticket invalidation and recapture continue according to their own settings.',
      on: 'Enabled: new mismatches automatically pause scheduling.',
      off: 'Disabled: new mismatches are recorded without automatically pausing.',
      loadFailed: 'Unable to read automatic quarantine settings. Please retry.',
      saveFailed: 'The save result could not be confirmed. Reload settings before trying again.'
    }
  }
}
