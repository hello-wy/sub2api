export default {
  accountTicketDefaults: {
    title: 'Automatic tickets for new accounts',
    off: 'Disable automatic acquisition',
    plan: 'Ticket plan',
    pro: 'Pro · 292',
    team: 'Team · 332',
    scope: 'Choose Pro or Team and save to enable automatic acquisition for future accounts, imports and new IP channels. This plan takes priority over group defaults, with gpt-6-astra as the verification model. Scheduling starts only after a ticket passes verification with the current credentials and fixed business IP.',
    futureOnly: 'Applies only to future accounts, imports and new IP channels. Existing account settings stay unchanged. When disabled, new accounts follow their existing group defaults.',
    prerequisites: 'The gateway STATE switch is off or the dynamic IP pool is missing. You can save the default now; new accounts will wait for configuration and verification.',
    prerequisitesUnknown: 'Gateway acquisition settings could not be loaded. You can still save the default; confirm acquisition readiness in gateway settings.',
    settings: 'Open gateway settings',
    saved: 'Default saved. Check each new account’s ticket status for its acquisition and verification results.',
    loadFailed: 'Could not load the default. Retry before saving.',
    saveFailed: 'Save failed; your choices have been retained. Retry or reopen to confirm the saved default.',
    reload: 'Reload',
    retryPrerequisites: 'Refresh gateway settings',
    save: 'Save default'
  }
}
