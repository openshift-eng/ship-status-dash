package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"ship-status-dash/pkg/auth"
	"ship-status-dash/pkg/types"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Handlers contains the HTTP request handlers for the dashboard API.
type Handlers struct {
	logger     *logrus.Logger
	config     *types.Config
	db         *gorm.DB
	groupCache *auth.GroupMembershipCache
}

// NewHandlers creates a new Handlers instance with the provided dependencies.
func NewHandlers(logger *logrus.Logger, config *types.Config, db *gorm.DB, groupCache *auth.GroupMembershipCache) *Handlers {
	return &Handlers{
		logger:     logger,
		config:     config,
		db:         db,
		groupCache: groupCache,
	}
}

func respondWithJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

func respondWithError(w http.ResponseWriter, statusCode int, message string) {
	respondWithJSON(w, statusCode, map[string]string{
		"error": message,
	})
}

func (h *Handlers) getComponent(componentName string) *types.Component {
	for _, component := range h.config.Components {
		if component.Slug == componentName {
			return component
		}
	}
	return nil
}

// IsUserAuthorizedForComponent checks if a user is authorized to perform mutating actions on a component.
// A user is authorized if they match any Owner.User field, or if they are a member of at least one rover_group configured for the component.
func (h *Handlers) IsUserAuthorizedForComponent(user string, component *types.Component) bool {
	for _, owner := range component.Owners {
		// Check if user matches the Owner.User field (for development/testing)
		if owner.User != "" && owner.User == user {
			return true
		}
		// Check if user is in any of the component's rover_groups
		if owner.RoverGroup != "" {
			if h.groupCache.IsUserInGroup(user, owner.RoverGroup) {
				return true
			}
		}
	}

	return false
}

func (h *Handlers) validateOutage(outage *types.Outage) (string, bool) {
	if outage.Severity == "" {
		return "Severity is required", false
	}
	if !types.IsValidSeverity(string(outage.Severity)) {
		return "Invalid severity. Must be one of: Down, Degraded, Suspected", false
	}
	if outage.StartTime.IsZero() {
		return "StartTime is required", false
	}
	if outage.DiscoveredFrom == "" {
		return "DiscoveredFrom is required", false
	}
	if outage.CreatedBy == "" {
		return "CreatedBy is required", false
	}
	return "", true
}

// HealthJSON returns the health status of the dashboard service.
func (h *Handlers) HealthJSON(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	}
	respondWithJSON(w, http.StatusOK, response)
}

// GetComponentsJSON returns the list of configured components.
func (h *Handlers) GetComponentsJSON(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, http.StatusOK, h.config.Components)
}

// GetComponentInfoJSON returns the information for a specific component.
func (h *Handlers) GetComponentInfoJSON(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	componentName := vars["componentName"]
	component := h.getComponent(componentName)
	if component == nil {
		respondWithError(w, http.StatusNotFound, "Component not found")
		return
	}
	respondWithJSON(w, http.StatusOK, component)
}

// GetOutagesJSON retrieves outages for a specific component, aggregating sub-component outages for top-level components.
func (h *Handlers) GetOutagesJSON(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	componentName := vars["componentName"]

	logger := h.logger.WithField("component", componentName)

	component := h.getComponent(componentName)
	if component == nil {
		respondWithError(w, http.StatusNotFound, "Component not found")
		return
	}
	subComponentSlugs := make([]string, len(component.Subcomponents))
	for i, subComponent := range component.Subcomponents {
		subComponentSlugs[i] = subComponent.Slug
	}

	var outages []types.Outage
	if err := h.db.Where("component_name = ? AND sub_component_name IN ?", componentName, subComponentSlugs).Order("start_time DESC").Find(&outages).Error; err != nil {
		logger.WithField("error", err).Error("Failed to query outages from database")
		respondWithError(w, http.StatusInternalServerError, "Failed to get outages")
		return
	}

	respondWithJSON(w, http.StatusOK, outages)
}

// GetSubComponentOutagesJSON retrieves outages for a specific sub-component.
func (h *Handlers) GetSubComponentOutagesJSON(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	componentName := vars["componentName"]
	subComponentName := vars["subComponentName"]

	logger := h.logger.WithFields(logrus.Fields{
		"component":     componentName,
		"sub_component": subComponentName,
	})

	component := h.getComponent(componentName)
	if component == nil {
		respondWithError(w, http.StatusNotFound, "Component not found")
		return
	}

	subComponent := component.GetSubComponentBySlug(subComponentName)
	if subComponent == nil {
		respondWithError(w, http.StatusNotFound, "Sub-component not found")
		return
	}

	var outages []types.Outage
	if err := h.db.Where("component_name = ? AND sub_component_name = ?", componentName, subComponentName).Order("start_time DESC").Find(&outages).Error; err != nil {
		logger.WithField("error", err).Error("Failed to query outages from database")
		respondWithError(w, http.StatusInternalServerError, "Failed to get outages")
		return
	}

	respondWithJSON(w, http.StatusOK, outages)
}

// CreateOutageJSON creates a new outage for a sub-component.
func (h *Handlers) CreateOutageJSON(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	componentName := vars["componentName"]
	subComponentName := vars["subComponentName"]

	activeUser, ok := GetUserFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "no active user found")
		return
	}

	logger := h.logger.WithFields(logrus.Fields{
		"component":     componentName,
		"sub_component": subComponentName,
		"active_user":   activeUser,
	})

	component := h.getComponent(componentName)
	if component == nil {
		respondWithError(w, http.StatusNotFound, "Component not found")
		return
	}
	subComponent := component.GetSubComponentBySlug(subComponentName)
	if subComponent == nil {
		respondWithError(w, http.StatusNotFound, "Sub-Component not found")
		return
	}

	if !h.IsUserAuthorizedForComponent(activeUser, component) {
		logger.Warn("User not authorized to create outage")
		respondWithError(w, http.StatusForbidden, "You are not authorized to perform this action on this component")
		return
	}

	var outage types.Outage
	if err := json.NewDecoder(r.Body).Decode(&outage); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	logger = logger.WithFields(logrus.Fields{
		"severity":        outage.Severity,
		"created_by":      outage.CreatedBy,
		"discovered_from": outage.DiscoveredFrom,
	})

	outage.ComponentName = componentName
	outage.SubComponentName = subComponentName
	outage.CreatedBy = activeUser

	// Auto-confirm outages when requires_confirmation is false
	if !subComponent.RequiresConfirmation {
		logger.Info("Auto-confirming outage as requires_confirmation is false")
		outage.ConfirmedBy = &activeUser
		outage.ConfirmedAt = sql.NullTime{Time: time.Now(), Valid: true}
	}

	if message, valid := h.validateOutage(&outage); !valid {
		respondWithError(w, http.StatusBadRequest, message)
		return
	}

	if err := h.db.Create(&outage).Error; err != nil {
		logger.WithField("error", err).Error("Failed to create outage in database")
		respondWithError(w, http.StatusInternalServerError, "Failed to create outage")
		return
	}

	logger.Infof("Successfully created outage: %d", outage.ID)

	respondWithJSON(w, http.StatusCreated, outage)
}

// UpdateOutageRequest represents the fields that can be updated in a PATCH request.
type UpdateOutageRequest struct {
	Severity    *string       `json:"severity,omitempty"`
	StartTime   *time.Time    `json:"start_time,omitempty"`
	EndTime     *sql.NullTime `json:"end_time,omitempty"`
	Description *string       `json:"description,omitempty"`
	// ResolvedBy should not be passed by the frontend, this is only for use via the component-monitor
	// the value will be obtained from the active user header otherwise
	ResolvedBy  *string `json:"resolved_by,omitempty"`
	Confirmed   *bool   `json:"confirmed,omitempty"`
	TriageNotes *string `json:"triage_notes,omitempty"`
}

// UpdateOutageJSON updates an existing outage with the provided fields.
func (h *Handlers) UpdateOutageJSON(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	componentName := vars["componentName"]
	subComponentName := vars["subComponentName"]
	outageIDStr := vars["outageId"]

	activeUser, ok := GetUserFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "no active user found")
		return
	}

	outageID, err := strconv.ParseUint(outageIDStr, 10, 32)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid outage ID")
		return
	}

	logger := h.logger.WithFields(logrus.Fields{
		"outage_id":     outageID,
		"component":     componentName,
		"sub_component": subComponentName,
		"active_user":   activeUser,
	})
	logger.Info("Updating outage")

	component := h.getComponent(componentName)
	if component == nil {
		respondWithError(w, http.StatusNotFound, "Component not found")
		return
	}

	subComponent := component.GetSubComponentBySlug(subComponentName)
	if subComponent == nil {
		respondWithError(w, http.StatusNotFound, "Sub-Component not found")
		return
	}

	if !h.IsUserAuthorizedForComponent(activeUser, component) {
		logger.Warn("User not authorized to update outage")
		respondWithError(w, http.StatusForbidden, "You are not authorized to perform this action on this component")
		return
	}

	var outage types.Outage
	if err := h.db.Where("id = ? AND component_name = ? AND sub_component_name = ?", uint(outageID), componentName, subComponentName).First(&outage).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondWithError(w, http.StatusNotFound, "Outage not found")
			return
		}
		logger.WithField("error", err).Error("Failed to query outage from database")
		respondWithError(w, http.StatusInternalServerError, "Failed to get outage")
		return
	}

	var updateReq UpdateOutageRequest
	if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if updateReq.Severity != nil {
		if !types.IsValidSeverity(*updateReq.Severity) {
			respondWithError(w, http.StatusBadRequest, "Invalid severity. Must be one of: Down, Degraded, Suspected")
			return
		}
		outage.Severity = types.Severity(*updateReq.Severity)
	}
	if updateReq.StartTime != nil {
		outage.StartTime = *updateReq.StartTime
	}
	if updateReq.EndTime != nil {
		if updateReq.EndTime.Time != outage.EndTime.Time {
			outage.EndTime = *updateReq.EndTime
			outage.ResolvedBy = &activeUser
		}
		if !updateReq.EndTime.Valid {
			outage.ResolvedBy = nil
		}
	}
	if updateReq.Description != nil {
		outage.Description = *updateReq.Description
	}
	if updateReq.ResolvedBy != nil {
		outage.ResolvedBy = updateReq.ResolvedBy
	}
	if updateReq.Confirmed != nil {
		if *updateReq.Confirmed && !outage.ConfirmedAt.Valid {
			outage.ConfirmedAt = sql.NullTime{Time: time.Now(), Valid: true}
			outage.ConfirmedBy = &activeUser
		} else if !*updateReq.Confirmed && outage.ConfirmedAt.Valid {
			outage.ConfirmedAt = sql.NullTime{Valid: false}
			outage.ConfirmedBy = nil
		}
	}
	if updateReq.TriageNotes != nil {
		outage.TriageNotes = updateReq.TriageNotes
	}

	if err := h.db.Save(&outage).Error; err != nil {
		logger.WithField("error", err).Error("Failed to update outage in database")
		respondWithError(w, http.StatusInternalServerError, "Failed to update outage")
		return
	}

	logger.Info("Successfully updated outage")

	respondWithJSON(w, http.StatusOK, outage)
}

// GetOutageJSON retrieves a specific outage by ID for a specific sub-component.
func (h *Handlers) GetOutageJSON(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	componentName := vars["componentName"]
	subComponentName := vars["subComponentName"]
	outageId := vars["outageId"]

	logger := h.logger.WithFields(logrus.Fields{
		"component":     componentName,
		"sub_component": subComponentName,
		"outage_id":     outageId,
	})

	component := h.getComponent(componentName)
	if component == nil {
		respondWithError(w, http.StatusNotFound, "Component not found")
		return
	}

	subComponent := component.GetSubComponentBySlug(subComponentName)
	if subComponent == nil {
		respondWithError(w, http.StatusNotFound, "Sub-component not found")
		return
	}

	var outage types.Outage
	if err := h.db.Where("id = ? AND component_name = ? AND sub_component_name = ?", outageId, componentName, subComponentName).First(&outage).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondWithError(w, http.StatusNotFound, "Outage not found")
			return
		}
		logger.WithField("error", err).Error("Failed to query outage from database")
		respondWithError(w, http.StatusInternalServerError, "Failed to get outage")
		return
	}

	logger.Info("Successfully retrieved outage")
	respondWithJSON(w, http.StatusOK, outage)
}

// DeleteOutage deletes an outage by ID for a specific sub-component.
func (h *Handlers) DeleteOutage(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	componentName := vars["componentName"]
	subComponentName := vars["subComponentName"]
	outageId := vars["outageId"]

	activeUser, ok := GetUserFromContext(r.Context())
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "no active user found")
		return
	}

	logger := h.logger.WithFields(logrus.Fields{
		"component":     componentName,
		"sub_component": subComponentName,
		"outage_id":     outageId,
		"active_user":   activeUser,
	})

	component := h.getComponent(componentName)
	if component == nil {
		respondWithError(w, http.StatusNotFound, "Component not found")
		return
	}

	subComponent := component.GetSubComponentBySlug(subComponentName)
	if subComponent == nil {
		respondWithError(w, http.StatusNotFound, "Sub-component not found")
		return
	}

	if !h.IsUserAuthorizedForComponent(activeUser, component) {
		logger.Warn("User not authorized to delete outage")
		respondWithError(w, http.StatusForbidden, "You are not authorized to perform this action on this component")
		return
	}

	var outage types.Outage
	if err := h.db.Where("id = ? AND component_name = ? AND sub_component_name = ?", outageId, componentName, subComponentName).First(&outage).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondWithError(w, http.StatusNotFound, "Outage not found")
			return
		}
		logger.WithField("error", err).Error("Failed to query outage from database")
		respondWithError(w, http.StatusInternalServerError, "Failed to get outage")
		return
	}

	if err := h.db.Delete(&outage).Error; err != nil {
		logger.WithField("error", err).Error("Failed to delete outage from database")
		respondWithError(w, http.StatusInternalServerError, "Failed to delete outage")
		return
	}

	logger.Info("Successfully deleted outage")
	w.WriteHeader(http.StatusNoContent)
}

// GetSubComponentStatusJSON returns the status of a subcomponent based on active outages
func (h *Handlers) GetSubComponentStatusJSON(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	componentName := vars["componentName"]
	subComponentName := vars["subComponentName"]

	logger := h.logger.WithFields(logrus.Fields{
		"component":     componentName,
		"sub_component": subComponentName,
	})

	component := h.getComponent(componentName)
	if component == nil {
		respondWithError(w, http.StatusNotFound, "Component not found")
		return
	}

	subComponent := component.GetSubComponentBySlug(subComponentName)
	if subComponent == nil {
		respondWithError(w, http.StatusNotFound, "Sub-component not found")
		return
	}

	var outages []types.Outage
	if err := h.db.Where("component_name = ? AND sub_component_name = ? AND (end_time IS NULL OR end_time > ?)", componentName, subComponentName, time.Now()).Order("start_time DESC").Find(&outages).Error; err != nil {
		logger.WithField("error", err).Error("Failed to query active outages from database")
		respondWithError(w, http.StatusInternalServerError, "Failed to get subcomponent status")
		return
	}

	status := types.StatusHealthy
	if len(outages) > 0 {
		status = determineStatusFromSeverity(outages)
	}

	response := types.ComponentStatus{
		ComponentName: fmt.Sprintf("%s/%s", componentName, subComponentName),
		Status:        status,
		ActiveOutages: outages,
	}
	respondWithJSON(w, http.StatusOK, response)
}

// GetComponentStatusJSON returns the status of a component based on active outages in all its sub-components
func (h *Handlers) GetComponentStatusJSON(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	componentName := vars["componentName"]

	logger := h.logger.WithField("component", componentName)

	component := h.getComponent(componentName)
	if component == nil {
		respondWithError(w, http.StatusNotFound, "Component not found")
		return
	}

	response, err := h.getComponentStatus(component, logger)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to get component status")
		return
	}
	respondWithJSON(w, http.StatusOK, response)
}

// GetAllComponentsStatusJSON returns the status of all components
func (h *Handlers) GetAllComponentsStatusJSON(w http.ResponseWriter, r *http.Request) {
	logger := h.logger

	var allComponentStatuses []types.ComponentStatus

	for _, component := range h.config.Components {
		componentLogger := logger.WithField("component", component.Name)
		componentStatus, err := h.getComponentStatus(component, componentLogger)
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to get component status")
			return
		}

		allComponentStatuses = append(allComponentStatuses, componentStatus)
	}
	respondWithJSON(w, http.StatusOK, allComponentStatuses)
}

// getComponentStatus calculates the status of a component based on its sub-components and active outages
func (h *Handlers) getComponentStatus(component *types.Component, logger *logrus.Entry) (types.ComponentStatus, error) {
	var outages []types.Outage
	if err := h.db.Where("component_name = ? AND (end_time IS NULL OR end_time > ?)", component.Slug, time.Now()).Order("start_time DESC").Find(&outages).Error; err != nil {
		logger.WithField("error", err).Error("Failed to query active outages from database")
		return types.ComponentStatus{}, err
	}

	subComponentsWithOutages := make(map[string]bool)
	for _, outage := range outages {
		subComponentsWithOutages[outage.SubComponentName] = true
	}

	var status types.Status
	if len(outages) == 0 {
		status = types.StatusHealthy
	} else if len(subComponentsWithOutages) < len(component.Subcomponents) {
		status = types.StatusPartial
	} else {
		status = determineStatusFromSeverity(outages)
	}

	return types.ComponentStatus{
		ComponentName: component.Name,
		Status:        status,
		ActiveOutages: outages,
	}, nil
}

func determineStatusFromSeverity(outages []types.Outage) types.Status {
	if len(outages) == 0 {
		return types.StatusHealthy
	}

	// First, determine status based on confirmed outages
	confirmedOutages := make([]types.Outage, 0)
	hasUnconfirmedOutage := false

	for _, outage := range outages {
		if outage.ConfirmedAt.Valid {
			confirmedOutages = append(confirmedOutages, outage)
		} else {
			hasUnconfirmedOutage = true
		}
	}

	// If there are confirmed outages, determine status by their severity
	if len(confirmedOutages) > 0 {
		mostCriticalSeverity := confirmedOutages[0].Severity
		highestLevel := types.GetSeverityLevel(mostCriticalSeverity)

		for _, outage := range confirmedOutages {
			level := types.GetSeverityLevel(outage.Severity)
			if level > highestLevel {
				highestLevel = level
				mostCriticalSeverity = outage.Severity
			}
		}
		return mostCriticalSeverity.ToStatus()
	}

	// Only unconfirmed outages - return Suspected
	if hasUnconfirmedOutage {
		return types.StatusSuspected
	}

	return types.StatusHealthy
}

type AuthenticatedUser struct {
	Username   string   `json:"username" yaml:"username"`
	Components []string `json:"components" yaml:"components"`
}

func (h *Handlers) GetAuthenticatedUserJSON(w http.ResponseWriter, r *http.Request) {
	user, authenticated := GetUserFromContext(r.Context())
	if !authenticated {
		respondWithError(w, http.StatusUnauthorized, "No Authenticated user found")
		return
	}

	response := AuthenticatedUser{
		Username:   user,
		Components: []string{},
	}

	// Return only components the user is authorized for
	for _, component := range h.config.Components {
		if h.IsUserAuthorizedForComponent(user, component) {
			response.Components = append(response.Components, component.Slug)
		}
	}

	respondWithJSON(w, http.StatusOK, response)
}
