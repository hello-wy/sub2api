package domain

// GroupCodexTicketDefaults is a template for newly created accounts only.
// It never overrides a pre-existing account's explicit ticket configuration.
type GroupCodexTicketDefaults struct {
	Enabled    bool   `json:"enabled"`
	TicketPlan string `json:"ticket_plan"`
	Model      string `json:"model"`
	// RequireVerified is an internal global-new-account policy, never trusted
	// from group JSON or backup data.
	RequireVerified bool `json:"-"`
}
