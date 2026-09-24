package testhelper

import (
	"database/sql"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"ship-status-dash/pkg/types"
)

// EquateNullTimeValidOnly compares sql.NullTime values by Valid only, ignoring Time.
var EquateNullTimeValidOnly = cmp.Comparer(func(a, b sql.NullTime) bool {
	return a.Valid == b.Valid
})

var outageFieldsAlwaysIgnored = []string{
	"StartTime",
	"DiscoveredFrom",
	"CreatedBy",
	"LastAuditableUpdate",
	"SubComponentName",
	"Reasons",
	"SlackThreads",
	"AuditLogs",
	"Reports",
	"TriageNotes",
	"Links",
	"Relationships",
	"CreatedAt",
	"UpdatedAt",
	"DeletedAt",
}

// OutageComparesEndTime reports whether want specifies fields that include EndTime comparison.
func OutageComparesEndTime(want types.Outage) bool {
	return want.ID != 0 ||
		want.ComponentName != "" ||
		want.Severity != "" ||
		want.Description != "" ||
		want.ConfirmedAt.Valid ||
		want.EndTime.Valid
}

// OutageCompareOptions returns cmp options for comparing outages in tests.
// Unset fields on want are ignored. When compareEndTime is false, EndTime is not compared.
func OutageCompareOptions(want types.Outage, compareEndTime bool) cmp.Options {
	ignore := append([]string{}, outageFieldsAlwaysIgnored...)
	if want.ID == 0 {
		ignore = append(ignore, "ID")
	}
	if want.ComponentName == "" {
		ignore = append(ignore, "ComponentName")
	}
	if want.Severity == "" {
		ignore = append(ignore, "Severity")
	}
	if want.Description == "" {
		ignore = append(ignore, "Description")
	}
	if !want.ConfirmedAt.Valid {
		ignore = append(ignore, "ConfirmedAt")
	}
	if !compareEndTime {
		ignore = append(ignore, "EndTime")
	}
	return cmp.Options{
		cmpopts.IgnoreFields(types.Outage{}, ignore...),
		EquateNullTimeValidOnly,
	}
}

// ReasonCompareOptions returns cmp options for comparing probe reasons in tests.
func ReasonCompareOptions() cmp.Options {
	return cmp.Options{
		cmpopts.IgnoreFields(types.Reason{}, "Model", "OutageID", "Links"),
	}
}

// OutageLinkCompareOptions returns cmp options for comparing outage links in tests.
// Unset fields on want are ignored.
func OutageLinkCompareOptions(want types.OutageLink) cmp.Options {
	ignore := []string{"Description", "CreatedAt", "UpdatedAt", "DeletedAt", "ID"}
	if want.OutageID == 0 {
		ignore = append(ignore, "OutageID")
	}
	if want.URL == "" {
		ignore = append(ignore, "URL")
	}
	if want.LinkType == "" {
		ignore = append(ignore, "LinkType")
	}
	return cmp.Options{
		cmpopts.IgnoreFields(types.OutageLink{}, ignore...),
	}
}
