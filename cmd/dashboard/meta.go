package main

import (
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"ship-status-dash/pkg/config"
	"ship-status-dash/pkg/outage"
	"ship-status-dash/pkg/types"
)

var metaRoutes = newMetaRouter()

func newMetaRouter() *mux.Router {
	r := mux.NewRouter()
	r.Path("/{componentSlug}/{subComponentSlug}/outages/{outageID}")
	r.Path("/{componentSlug}/{subComponentSlug}")
	r.Path("/{componentSlug}")
	return r
}

type pageMetadata struct {
	Title       string
	Description string
}

const defaultTitle = "SHIP Status Dashboard"
const defaultDescription = "Real-time status monitoring and availability tracking for OpenShift CI components."

func defaultMetadata() pageMetadata {
	return pageMetadata{Title: defaultTitle, Description: defaultDescription}
}

func resolveMetadata(r *http.Request, configManager *config.Manager[types.DashboardConfig], outageManager outage.OutageManager, logger *logrus.Logger) pageMetadata {
	path := strings.TrimRight(r.URL.Path, "/")
	if path == "" {
		return defaultMetadata()
	}

	segments := strings.Split(strings.TrimPrefix(path, "/"), "/")

	switch segments[0] {
	case "status-history":
		return pageMetadata{
			Title:       "Status History - SHIP Status Dashboard",
			Description: "Historical status timeline for all OpenShift CI components.",
		}
	case "pages":
		if len(segments) >= 2 {
			return pageMetadata{
				Title:       fmt.Sprintf("%s - SHIP Status Dashboard", deslugify(segments[1])),
				Description: fmt.Sprintf("View the %s page on SHIP Status Dashboard.", deslugify(segments[1])),
			}
		}
	case "tags":
		if len(segments) >= 2 {
			return pageMetadata{
				Title:       fmt.Sprintf("Tag: %s - SHIP Status Dashboard", deslugify(segments[1])),
				Description: fmt.Sprintf("Components tagged with %s on SHIP Status Dashboard.", deslugify(segments[1])),
			}
		}
	case "team":
		if len(segments) >= 2 {
			return pageMetadata{
				Title:       fmt.Sprintf("Team: %s - SHIP Status Dashboard", deslugify(segments[1])),
				Description: fmt.Sprintf("Components managed by %s on SHIP Status Dashboard.", deslugify(segments[1])),
			}
		}
	}

	if configManager == nil {
		return defaultMetadata()
	}
	cfg := configManager.Get()
	if cfg == nil {
		return defaultMetadata()
	}

	var match mux.RouteMatch
	matchReq, _ := http.NewRequest(http.MethodGet, path, nil)
	if metaRoutes.Match(matchReq, &match) {
		vars := match.Vars
		switch {
		case vars["outageID"] != "":
			return resolveOutageMetadata(vars["componentSlug"], vars["subComponentSlug"], vars["outageID"], cfg, outageManager, logger)
		case vars["subComponentSlug"] != "":
			return resolveSubComponentMetadata(vars["componentSlug"], vars["subComponentSlug"], cfg)
		case vars["componentSlug"] != "":
			return resolveComponentMetadata(vars["componentSlug"], cfg)
		}
	}

	return defaultMetadata()
}

func resolveComponentMetadata(slug string, cfg *types.DashboardConfig) pageMetadata {
	comp := cfg.GetComponentBySlug(slug)
	if comp == nil {
		return defaultMetadata()
	}
	desc := comp.Description
	if desc == "" {
		desc = fmt.Sprintf("Status and outage information for %s.", comp.Name)
	}
	return pageMetadata{
		Title:       fmt.Sprintf("%s - SHIP Status Dashboard", comp.Name),
		Description: desc,
	}
}

func resolveSubComponentMetadata(compSlug, subSlug string, cfg *types.DashboardConfig) pageMetadata {
	comp := cfg.GetComponentBySlug(compSlug)
	if comp == nil {
		return defaultMetadata()
	}
	sub := comp.GetSubComponentBySlug(subSlug)
	if sub == nil {
		return pageMetadata{
			Title:       fmt.Sprintf("%s - SHIP Status Dashboard", comp.Name),
			Description: comp.Description,
		}
	}
	desc := sub.Description
	if desc == "" {
		desc = fmt.Sprintf("Status and outage information for %s (%s).", sub.Name, comp.Name)
	}
	return pageMetadata{
		Title:       fmt.Sprintf("%s (%s) - SHIP Status Dashboard", sub.Name, comp.Name),
		Description: desc,
	}
}

func resolveOutageMetadata(compSlug, subSlug, outageIDStr string, cfg *types.DashboardConfig, outageManager outage.OutageManager, logger *logrus.Logger) pageMetadata {
	compName := deslugify(compSlug)
	subName := deslugify(subSlug)
	if comp := cfg.GetComponentBySlug(compSlug); comp != nil {
		compName = comp.Name
		if sub := comp.GetSubComponentBySlug(subSlug); sub != nil {
			subName = sub.Name
		}
	}

	outageID, err := strconv.ParseUint(outageIDStr, 10, 64)
	if err != nil {
		return pageMetadata{
			Title:       fmt.Sprintf("Outage - %s (%s) - SHIP Status Dashboard", subName, compName),
			Description: fmt.Sprintf("Outage details for %s, a sub-component of %s.", subName, compName),
		}
	}

	fallback := pageMetadata{
		Title:       fmt.Sprintf("Outage #%d - %s (%s) - SHIP Status Dashboard", outageID, subName, compName),
		Description: fmt.Sprintf("Outage details for %s, a sub-component of %s.", subName, compName),
	}

	if outageManager == nil {
		return fallback
	}

	o, err := outageManager.GetOutageByID(compSlug, subSlug, uint(outageID))
	if err != nil {
		if logger != nil {
			logger.WithError(err).WithField("outageID", outageID).Debug("Failed to fetch outage for metadata")
		}
		return fallback
	}

	var status string
	if o.EndTime.Valid {
		status = "Resolved"
	} else {
		status = "Active"
	}

	desc := fmt.Sprintf("%s | Severity: %s | %s (%s)", status, string(o.Severity), subName, compName)
	if o.Description != "" {
		d := o.Description
		if len(d) > 150 {
			d = d[:147] + "..."
		}
		desc += " | " + d
	}

	return pageMetadata{
		Title:       fmt.Sprintf("Outage #%d - %s (%s) - SHIP Status Dashboard", outageID, subName, compName),
		Description: desc,
	}
}

func deslugify(slug string) string {
	words := strings.Split(slug, "-")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

func injectMetadata(indexHTML []byte, meta pageMetadata) []byte {
	content := string(indexHTML)

	safeTitle := html.EscapeString(meta.Title)
	safeDesc := html.EscapeString(meta.Description)

	content = strings.Replace(content,
		"<title>SHIP Status Dashboard</title>",
		fmt.Sprintf("<title>%s</title>", safeTitle),
		1)

	content = strings.Replace(content,
		`<meta name="description" content="SHIP Status Dashboard" />`,
		fmt.Sprintf(`<meta name="description" content="%s" />`, safeDesc),
		1)

	ogTags := fmt.Sprintf(`    <meta property="og:title" content="%s" />
    <meta property="og:description" content="%s" />
    <meta property="og:type" content="website" />
    <meta property="og:site_name" content="SHIP Status Dashboard" />
`, safeTitle, safeDesc)

	content = strings.Replace(content, "  </head>", ogTags+"  </head>", 1)

	return []byte(content)
}
