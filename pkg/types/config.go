package types

// Config contains the application configuration including component definitions.
type Config struct {
	Components []*Component `json:"components" yaml:"components"`
}

// Component represents a top-level system component with sub-components and ownership information.
type Component struct {
	Name          string         `json:"name" yaml:"name"`
	Slug          string         `json:"slug"`
	Description   string         `json:"description" yaml:"description"`
	ShipTeam      string         `json:"ship_team" yaml:"ship_team"`
	SlackChannel  string         `json:"slack_channel" yaml:"slack_channel"`
	Subcomponents []SubComponent `json:"sub_components" yaml:"sub_components"`
	Owners        []Owner        `json:"owners" yaml:"owners"`
}

func (c *Component) GetSubComponentBySlug(slug string) *SubComponent {
	for i := range c.Subcomponents {
		if c.Subcomponents[i].Slug == slug {
			return &c.Subcomponents[i]
		}
	}
	return nil
}

// SubComponent represents a sub-component that can have outages tracked against it.
type SubComponent struct {
	Name                 string `json:"name" yaml:"name"`
	Slug                 string `json:"slug"`
	Description          string `json:"description" yaml:"description"`
	Managed              bool   `json:"managed" yaml:"managed"`
	RequiresConfirmation bool   `json:"requires_confirmation" yaml:"requires_confirmation"`
}

// Owner represents ownership information for a component, either via Rover group or service account.
type Owner struct {
	RoverGroup     string `json:"rover_group,omitempty" yaml:"rover_group,omitempty"`
	ServiceAccount string `json:"service_account,omitempty" yaml:"service_account,omitempty"`
}
