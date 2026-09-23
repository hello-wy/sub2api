package service

// Defaults apply only when creating a new routing record. Existing account
// limits, including an explicitly configured zero, are never reset by an edit.
const DefaultIPChannelConcurrency = 50
const DefaultAccountPriority = 1
