package types

import (
	"fmt"
	"strings"

	apimachineryerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
)

// SLOComponentView is one flagged component's active outages on the SLO page.
type SLOComponentView struct {
	Component    string   `json:"component"`
	SubComponent string   `json:"sub_component"`
	Outages      []Outage `json:"outages"`
}

// ValidateTeamSLOs checks team SLO YAML.
// known reports whether this build accepts the workspace schema.
// validateSpec checks schema-specific settings and may be nil.
// Unknown evaluator sources are allowed and ignored at read time.
func (c *DashboardConfig) ValidateTeamSLOs(known func(kind string, version int) bool, validateSpec func(*SLOWorkspace) error) error {
	var errs []error
	seenTeams := sets.New[string]()
	listedSLOComponents := map[string]string{}
	for i := range c.TeamSLOs {
		team := &c.TeamSLOs[i]
		name := strings.TrimSpace(team.Team)
		if name == "" {
			errs = append(errs, fmt.Errorf("team_slos[%d]: team is required", i))
			continue
		}
		if seenTeams.Has(name) {
			errs = append(errs, fmt.Errorf("team_slos: duplicate team %q", name))
			continue
		}
		seenTeams.Insert(name)
		if len(team.Owners) == 0 {
			errs = append(errs, fmt.Errorf("team_slos %q: owners is required", name))
		} else if !ownersPresent(team.Owners) {
			errs = append(errs, fmt.Errorf("team_slos %q: owners must include a user, service account, or rover group", name))
		}
		seenSLOComponent := sets.New[string]()
		for _, slug := range team.SLOComponents {
			if strings.TrimSpace(slug) == "" {
				errs = append(errs, fmt.Errorf("team_slos %q: slo_component slug is required", name))
				continue
			}
			if seenSLOComponent.Has(slug) {
				errs = append(errs, fmt.Errorf("team_slos %q: duplicate slo_component %q", name, slug))
				continue
			}
			seenSLOComponent.Insert(slug)
			if listedOn, ok := listedSLOComponents[slug]; ok {
				errs = append(errs, fmt.Errorf("slo_component %q is listed on both %q and %q", slug, listedOn, name))
				continue
			}
			listedSLOComponents[slug] = name
			component := c.GetComponentBySlug(slug)
			if component == nil {
				errs = append(errs, fmt.Errorf("team_slos %q: slo_component %q does not exist", name, slug))
				continue
			}
			if !component.SLOComponent {
				errs = append(errs, fmt.Errorf("team_slos %q: component %q is not marked slo_component", name, slug))
			}
		}
		seenSLO := sets.New[string]()
		var workspace *SLOWorkspace
		for j := range team.SLOs {
			slo := &team.SLOs[j]
			if strings.TrimSpace(slo.Name) == "" {
				errs = append(errs, fmt.Errorf("team_slos %q: slo name is required", name))
				continue
			}
			if seenSLO.Has(slo.Name) {
				errs = append(errs, fmt.Errorf("team_slos %q: duplicate slo %q", name, slo.Name))
				continue
			}
			seenSLO.Insert(slo.Name)
			if strings.TrimSpace(slo.Source) == "" {
				errs = append(errs, fmt.Errorf("team_slos %q slo %q: source is required", name, slo.Name))
			}
			if slo.Workspace == nil {
				continue
			}
			if workspace != nil {
				errs = append(errs, fmt.Errorf("team_slos %q: only one workspace is allowed", name))
				continue
			}
			workspace = slo.Workspace
			if err := validateWorkspaceIdentity(name, workspace, known, validateSpec); err != nil {
				errs = append(errs, err)
			}
		}
	}
	for _, component := range c.Components {
		if !component.SLOComponent {
			continue
		}
		slug := component.Slug
		if _, ok := listedSLOComponents[slug]; !ok {
			errs = append(errs, fmt.Errorf("component %q is marked slo_component but is not listed in team_slos", slug))
		}
	}
	return apimachineryerrors.NewAggregate(errs)
}

func validateWorkspaceIdentity(team string, ws *SLOWorkspace, known func(kind string, version int) bool, validateSpec func(*SLOWorkspace) error) error {
	if strings.TrimSpace(ws.Kind) == "" {
		return fmt.Errorf("team_slos %q: workspace.kind is required", team)
	}
	if ws.SchemaVersion <= 0 {
		return fmt.Errorf("team_slos %q: workspace.schema_version is required", team)
	}
	if known != nil && !known(ws.Kind, ws.SchemaVersion) {
		return fmt.Errorf("team_slos %q: unknown workspace schema %s v%d", team, ws.Kind, ws.SchemaVersion)
	}
	if validateSpec != nil {
		if err := validateSpec(ws); err != nil {
			return fmt.Errorf("team_slos %q: %w", team, err)
		}
	}
	return nil
}

func ownersPresent(owners []Owner) bool {
	for _, owner := range owners {
		if strings.TrimSpace(owner.User) != "" || strings.TrimSpace(owner.ServiceAccount) != "" || strings.TrimSpace(owner.RoverGroup) != "" {
			return true
		}
	}
	return false
}

// SummaryTeamNames is the team_slos team list.
func (c *DashboardConfig) SummaryTeamNames() []string {
	seen := sets.New[string]()
	var names []string
	for _, slo := range c.TeamSLOs {
		if slo.Team == "" || seen.Has(slo.Team) {
			continue
		}
		seen.Insert(slo.Team)
		names = append(names, slo.Team)
	}
	return names
}

// SLOComponentsForTeam lists components named on the team's slo_components list.
func (c *DashboardConfig) SLOComponentsForTeam(team string, outagesByRef map[SubComponentRef][]Outage) []SLOComponentView {
	teamCfg := c.TeamSLOByTeam(team)
	if teamCfg == nil {
		return []SLOComponentView{}
	}
	var out []SLOComponentView
	for _, slug := range teamCfg.SLOComponents {
		component := c.GetComponentBySlug(slug)
		if component == nil || !component.SLOComponent {
			continue
		}
		for i := range component.Subcomponents {
			sub := &component.Subcomponents[i]
			ref := SubComponentRef{ComponentSlug: component.Slug, SubSlug: sub.Slug}
			outages := outagesByRef[ref]
			if outages == nil {
				outages = []Outage{}
			}
			out = append(out, SLOComponentView{
				Component:    component.Name,
				SubComponent: sub.Name,
				Outages:      outages,
			})
		}
	}
	if out == nil {
		out = []SLOComponentView{}
	}
	return out
}
