package outage

import (
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"

	"ship-status-dash/pkg/config"
	"ship-status-dash/pkg/repositories"
	"ship-status-dash/pkg/types"
	"ship-status-dash/pkg/utils"
)

// SlackReporter handles Slack reporting for outages.
type SlackReporter struct {
	slackClient       *slack.Client
	slackThreadRepo   repositories.SlackThreadRepository
	configManager     *config.Manager[types.DashboardConfig]
	baseURL           string
	slackWorkspaceURL string
	logger            *logrus.Logger
}

// NewSlackReporter creates a new SlackReporter instance.
func NewSlackReporter(
	slackClient *slack.Client,
	slackThreadRepo repositories.SlackThreadRepository,
	configManager *config.Manager[types.DashboardConfig],
	baseURL string,
	slackWorkspaceURL string,
	logger *logrus.Logger,
) *SlackReporter {
	normalizedBaseURL := baseURL
	if normalizedBaseURL != "" && !strings.HasSuffix(normalizedBaseURL, "/") {
		normalizedBaseURL += "/"
	}
	normalizedWorkspaceURL := slackWorkspaceURL
	if normalizedWorkspaceURL != "" && !strings.HasSuffix(normalizedWorkspaceURL, "/") {
		normalizedWorkspaceURL += "/"
	}
	return &SlackReporter{
		slackClient:       slackClient,
		slackThreadRepo:   slackThreadRepo,
		configManager:     configManager,
		baseURL:           normalizedBaseURL,
		slackWorkspaceURL: normalizedWorkspaceURL,
		logger:            logger,
	}
}

// ReportOutage reports a new outage to Slack channels.
func (r *SlackReporter) ReportOutage(outage *types.Outage) error {
	reporting := r.getSlackReportingForSubComponent(outage.ComponentName, outage.SubComponentName)
	if len(reporting) == 0 {
		return nil
	}

	channels := filterChannelsBySeverity(reporting, outage.Severity)
	if len(channels) == 0 {
		return nil
	}

	cfg := r.configManager.Get()
	component := cfg.GetComponentBySlug(outage.ComponentName)
	if component == nil {
		return fmt.Errorf("component not found: %s", outage.ComponentName)
	}

	message := r.formatOutageMessage(outage, component)

	return r.postToSlackChannels(outage, channels, message)
}

// ReportOutageUpdate reports an outage update to existing Slack threads.
func (r *SlackReporter) ReportOutageUpdate(outage *types.Outage, oldOutage *types.Outage) error {
	threads, err := r.slackThreadRepo.GetThreadsForOutage(outage.ID)
	if err != nil {
		r.logger.WithFields(logrus.Fields{
			"outage_id": outage.ID,
			"error":     err,
		}).Warn("Failed to get Slack threads for outage")
		return err
	}

	// If we don't have any threads for this outage, the severity has either been upgraded, or we were missing the initial report.
	// We should report it as new to ensure that the correct channels are notified.
	if len(threads) == 0 {
		return r.ReportOutage(outage)
	}

	message := r.formatUpdateMessage(outage, oldOutage)

	return r.replyToSlackThreads(outage, threads, message)
}

func (r *SlackReporter) getSlackReportingForSubComponent(componentSlug, subComponentSlug string) []types.SlackReportingConfig {
	cfg := r.configManager.Get()
	component := cfg.GetComponentBySlug(componentSlug)
	if component == nil {
		return nil
	}

	subComponent := component.GetSubComponentBySlug(subComponentSlug)
	if subComponent == nil {
		return nil
	}

	return types.GetSlackReporting(component, subComponent)
}

func getEffectiveSeverity(config types.SlackReportingConfig) types.Severity {
	if config.Severity != nil && *config.Severity != "" {
		return *config.Severity
	}
	return types.SeveritySuspected
}

func filterChannelsBySeverity(reporting []types.SlackReportingConfig, outageSeverity types.Severity) []string {
	var channels []string
	outageLevel := types.GetSeverityLevel(outageSeverity)

	for _, config := range reporting {
		effectiveSeverity := getEffectiveSeverity(config)
		configLevel := types.GetSeverityLevel(effectiveSeverity)

		if outageLevel >= configLevel {
			channels = append(channels, config.Channel)
		}
	}

	return channels
}

func (r *SlackReporter) buildOutageLink(outage *types.Outage) string {
	componentSlug := utils.Slugify(outage.ComponentName)
	subComponentSlug := utils.Slugify(outage.SubComponentName)
	return fmt.Sprintf("%s%s/%s/outages/%d", r.baseURL, componentSlug, subComponentSlug, outage.ID)
}

func (r *SlackReporter) buildThreadURL(channel string, timestamp string) string {
	channelID := strings.TrimPrefix(channel, "#")
	timestampID := strings.ReplaceAll(timestamp, ".", "")
	return fmt.Sprintf("%sarchives/%s/p%s", r.slackWorkspaceURL, channelID, timestampID)
}

func (r *SlackReporter) addResolvedEmoji(channelID string, timestamp string) error {
	itemRef := slack.NewRefToMessage(channelID, timestamp)
	return r.slackClient.AddReaction("outage_resolved", itemRef)
}

func (r *SlackReporter) formatOutageMessage(outage *types.Outage, component *types.Component) string {
	var parts []string
	subComponentName := outage.SubComponentName
	if subComponent := component.GetSubComponentBySlug(outage.SubComponentName); subComponent != nil {
		subComponentName = subComponent.Name
	}
	parts = append(parts, fmt.Sprintf("🚨 Outage Detected: %s/%s", component.Name, subComponentName))
	parts = append(parts, "")
	parts = append(parts, fmt.Sprintf("Severity: `%s`", outage.Severity))
	if outage.Description != "" {
		description := truncateString(outage.Description)
		parts = append(parts, "Description:")
		parts = append(parts, formatQuoteBlock(description))
	}
	if triageNotes := getStringValue(outage.TriageNotes, ""); triageNotes != "" {
		truncatedNotes := truncateString(triageNotes)
		parts = append(parts, "Triage notes:")
		parts = append(parts, formatQuoteBlock(truncatedNotes))
	}
	parts = append(parts, fmt.Sprintf("Started: `%s`", outage.StartTime.Format(time.RFC3339)))
	parts = append(parts, fmt.Sprintf("Created by: `%s`", outage.CreatedBy))
	parts = append(parts, fmt.Sprintf("Discovered from: `%s`", outage.DiscoveredFrom))
	parts = append(parts, "")
	parts = append(parts, fmt.Sprintf("<%s|View Outage>", r.buildOutageLink(outage)))

	return strings.Join(parts, "\n")
}

func (r *SlackReporter) formatUpdateMessage(outage *types.Outage, oldOutage *types.Outage) string {
	var parts []string
	component := r.configManager.Get().GetComponentBySlug(outage.ComponentName)
	componentName := "Unknown"
	if component != nil {
		componentName = component.Name
	}

	subComponentName := outage.SubComponentName
	if component != nil {
		if subComponent := component.GetSubComponentBySlug(outage.SubComponentName); subComponent != nil {
			subComponentName = subComponent.Name
		}
	}
	emoji := "📝"
	if !oldOutage.EndTime.Valid && outage.EndTime.Valid {
		// Resolved emoji is only used when the outage is first resolved
		emoji = ":outage_resolved:"
	}
	parts = append(parts, fmt.Sprintf("%s Outage Updated: %s/%s (#%d)", emoji, componentName, subComponentName, outage.ID))
	parts = append(parts, "")

	var changes []string

	if oldOutage.Severity != outage.Severity {
		changes = append(changes, fmt.Sprintf("Severity changed: `%s` → `%s`", oldOutage.Severity, outage.Severity))
	}

	if oldOutage.EndTime.Valid != outage.EndTime.Valid {
		if outage.EndTime.Valid {
			changes = append(changes, fmt.Sprintf("Resolved: by `%s` at `%s`", getStringValue(outage.ResolvedBy, "Unknown"), outage.EndTime.Time.Format(time.RFC3339)))
		} else {
			changes = append(changes, "Reopened")
		}
	} else if oldOutage.EndTime.Valid && outage.EndTime.Valid {
		if !oldOutage.EndTime.Time.Equal(outage.EndTime.Time) {
			changes = append(changes, fmt.Sprintf("Resolved time updated: `%s`", outage.EndTime.Time.Format(time.RFC3339)))
		}
	}

	if oldOutage.ConfirmedAt.Valid != outage.ConfirmedAt.Valid {
		if outage.ConfirmedAt.Valid {
			changes = append(changes, fmt.Sprintf("Confirmed: `%s` at `%s`", getStringValue(outage.ConfirmedBy, "Unknown"), outage.ConfirmedAt.Time.Format(time.RFC3339)))
		} else {
			changes = append(changes, "Unconfirmed")
		}
	}

	if oldOutage.Description != outage.Description {
		description := truncateString(outage.Description)
		changes = append(changes, "Description updated:")
		changes = append(changes, formatQuoteBlock(description))
	}

	if getStringValue(oldOutage.TriageNotes, "") != getStringValue(outage.TriageNotes, "") {
		triageNotes := truncateString(getStringValue(outage.TriageNotes, ""))
		changes = append(changes, "Triage notes updated:")
		changes = append(changes, formatQuoteBlock(triageNotes))
	}

	if len(changes) == 0 {
		changes = append(changes, "Outage updated")
	}

	parts = append(parts, strings.Join(changes, "\n"))
	parts = append(parts, "")
	parts = append(parts, fmt.Sprintf("<%s|View Outage>", r.buildOutageLink(outage)))

	return strings.Join(parts, "\n")
}

func getStringValue(ptr *string, defaultValue string) string {
	if ptr == nil {
		return defaultValue
	}
	return *ptr
}

const maxTruncateLength = 240

func truncateString(s string) string {
	if len(s) <= maxTruncateLength {
		return s
	}
	return s[:maxTruncateLength-3] + "..."
}

func formatQuoteBlock(text string) string {
	lines := strings.Split(text, "\n")
	var quotedLines []string
	for _, line := range lines {
		quotedLines = append(quotedLines, fmt.Sprintf(">%s", line))
	}
	return strings.Join(quotedLines, "\n")
}

func (r *SlackReporter) postToSlackChannels(outage *types.Outage, channels []string, message string) error {
	cfg := r.configManager.Get()
	component := cfg.GetComponentBySlug(outage.ComponentName)
	if component == nil {
		return fmt.Errorf("component not found: %s", outage.ComponentName)
	}

	var lastErr error
	for _, channel := range channels {
		logger := r.logger.WithFields(logrus.Fields{
			"outage_id": outage.ID,
			"channel":   channel,
		})

		channelID, timestamp, err := r.slackClient.PostMessage(
			channel,
			slack.MsgOptionText(message, false),
			slack.MsgOptionAsUser(true),
		)

		if err != nil {
			logger.WithField("error", err).Error("Failed to post message to Slack")
			lastErr = err
			continue
		}

		threadURL := r.buildThreadURL(channel, timestamp)
		thread := &types.SlackThread{
			OutageID:        outage.ID,
			Channel:         channel,
			ChannelID:       channelID,
			ThreadTimestamp: timestamp,
			ThreadURL:       threadURL,
		}

		if err := r.slackThreadRepo.CreateThread(thread); err != nil {
			logger.WithField("error", err).Error("Failed to store Slack thread timestamp")
			lastErr = err
			continue
		}

		if outage.EndTime.Valid {
			if err := r.addResolvedEmoji(channelID, timestamp); err != nil {
				logger.WithField("error", err).Warn("Failed to add resolved emoji to message")
			}
		}

		logger.Info("Successfully posted outage to Slack")
	}

	return lastErr
}

func (r *SlackReporter) replyToSlackThreads(outage *types.Outage, threads []types.SlackThread, message string) error {
	var lastErr error
	for _, thread := range threads {
		logger := r.logger.WithFields(logrus.Fields{
			"outage_id":        outage.ID,
			"channel":          thread.Channel,
			"thread_timestamp": thread.ThreadTimestamp,
		})

		_, _, err := r.slackClient.PostMessage(
			thread.Channel,
			slack.MsgOptionText(message, false),
			slack.MsgOptionTS(thread.ThreadTimestamp),
			slack.MsgOptionAsUser(true),
		)

		if err != nil {
			logger.WithField("error", err).Error("Failed to post thread reply to Slack")
			lastErr = err
			continue
		}

		if outage.EndTime.Valid {
			if err := r.addResolvedEmoji(thread.ChannelID, thread.ThreadTimestamp); err != nil {
				logger.WithField("error", err).Warn("Failed to add resolved emoji to message")
			}
		}

		logger.Info("Successfully posted outage update to Slack thread")
	}

	return lastErr
}
