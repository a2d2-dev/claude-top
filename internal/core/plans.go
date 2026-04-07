package core

// Plan defines a Claude subscription plan with its limits.
type Plan struct {
	// Name is the canonical plan identifier.
	Name string
	// DisplayName is the human-readable name shown in the UI.
	DisplayName string
	// TokenLimit is the maximum tokens per 5-hour session window.
	TokenLimit int
	// CostLimit is the approximate USD spend limit per window.
	CostLimit float64
	// MessageLimit is the maximum messages per window.
	MessageLimit int
}

// predefined plans matching the Python reference implementation.
var predefinedPlans = map[string]Plan{
	"pro": {
		Name:         "pro",
		DisplayName:  "Pro",
		TokenLimit:   19_000,
		CostLimit:    18.0,
		MessageLimit: 250,
	},
	"max5": {
		Name:         "max5",
		DisplayName:  "Max 5×",
		TokenLimit:   88_000,
		CostLimit:    35.0,
		MessageLimit: 1_000,
	},
	"max20": {
		Name:         "max20",
		DisplayName:  "Max 20×",
		TokenLimit:   220_000,
		CostLimit:    140.0,
		MessageLimit: 2_000,
	},
}

// DefaultPlan is used when no plan is specified.
const DefaultPlan = "pro"

// GetPlan returns the Plan for the given name, or the default plan if unknown.
func GetPlan(name string) Plan {
	if p, ok := predefinedPlans[name]; ok {
		return p
	}
	return predefinedPlans[DefaultPlan]
}

// AllPlans returns a slice of all known plans in order.
func AllPlans() []Plan {
	return []Plan{
		predefinedPlans["pro"],
		predefinedPlans["max5"],
		predefinedPlans["max20"],
	}
}
