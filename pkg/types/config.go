package types

// Config contains the application configuration including component definitions.
type Config struct {
	Components []*Component `json:"components" yaml:"components"`
}

func (c *Config) GetComponentBySlug(slug string) *Component {
	for i := range c.Components {
		if c.Components[i].Slug == slug {
			return c.Components[i]
		}
	}
	return nil
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
	Name                 string     `json:"name" yaml:"name"`
	Slug                 string     `json:"slug"`
	Description          string     `json:"description" yaml:"description"`
	Monitoring           Monitoring `json:"monitoring" yaml:"monitoring"`
	RequiresConfirmation bool       `json:"requires_confirmation" yaml:"requires_confirmation"`
}

// Monitoring defines how this sub-component is automatically monitored.
type Monitoring struct {
	Frequency        string `json:"frequency" yaml:"frequency"`
	ComponentMonitor string `json:"component_monitor" yaml:"component_monitor"`
	// AutoResolve is a flag that indicates whether outages discovered by the component-monitor should be automatically resolved when
	// the component-monitor reports the sub-component is healthy.
	AutoResolve bool `json:"auto_resolve" yaml:"auto_resolve"`
}

// Owner represents ownership information for a component, either via Rover group or service account.
type Owner struct {
	RoverGroup     string `json:"rover_group,omitempty" yaml:"rover_group,omitempty"`
	ServiceAccount string `json:"service_account,omitempty" yaml:"service_account,omitempty"`
	// User is a username of a user who is an admin of the component, this is used for development/testing purposes only
	User string `json:"user,omitempty" yaml:"user,omitempty"`
}
