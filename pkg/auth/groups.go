package auth

import (
	"context"
	"fmt"
	"slices"

	userv1client "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// GroupMembershipCache stores the mapping of rover_group names to their member users.
type GroupMembershipCache struct {
	groups map[string][]string
	logger *logrus.Logger
}

// NewGroupMembershipCache creates a new empty group membership cache.
func NewGroupMembershipCache(logger *logrus.Logger) *GroupMembershipCache {
	return &GroupMembershipCache{
		groups: make(map[string][]string),
		logger: logger,
	}
}

// LoadGroups queries OpenShift Groups API for the specified group names and populates the cache.
func (c *GroupMembershipCache) LoadGroups(groupNames []string, kubeconfigPath string) error {
	var config *rest.Config
	var err error

	if kubeconfigPath == "" {
		// In-cluster config uses the service account token automatically
		config, err = rest.InClusterConfig()
		if err != nil {
			return fmt.Errorf("failed to build in-cluster config: %w", err)
		}
	} else {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return fmt.Errorf("failed to build kubeconfig: %w", err)
		}
	}

	userClient, err := userv1client.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create user client: %w", err)
	}

	ctx := context.Background()
	for _, groupName := range groupNames {
		group, err := userClient.Groups().Get(ctx, groupName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get group %s: %w", groupName, err)
		}

		users := make([]string, len(group.Users))
		copy(users, group.Users)
		c.groups[groupName] = users

		c.logger.WithFields(logrus.Fields{
			"group":      groupName,
			"user_count": len(users),
		}).Info("Loaded group membership")
	}

	return nil
}

// IsUserInGroup checks if a user is a member of the specified group.
func (c *GroupMembershipCache) IsUserInGroup(user, groupName string) bool {
	users, exists := c.groups[groupName]
	if !exists {
		return false
	}

	return slices.Contains(users, user)
}

// GetGroupMembers returns the list of users in the specified group.
func (c *GroupMembershipCache) GetGroupMembers(groupName string) []string {
	users, exists := c.groups[groupName]
	if !exists {
		return nil
	}
	return users
}
