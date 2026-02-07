package webhooks

// Payload is the structure sent to webhook endpoints
type Payload struct {
	Endpoint   string `json:"endpoint"`
	UserID     string `json:"user_id"`
	ActivePlan string `json:"active_plan"`
	Meta       Meta   `json:"meta"`
}

// Meta contains additional context about the plan update
type Meta struct {
	PrevPlan     string `json:"prev_plan"`
	Subscription any    `json:"subscription"` // Full subscription object
}
