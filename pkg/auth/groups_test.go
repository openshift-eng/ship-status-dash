package auth

import (
	"testing"

	"github.com/sirupsen/logrus"
)

func TestNewGroupMembershipCache(t *testing.T) {
	logger := logrus.New()
	cache := NewGroupMembershipCache(logger)

	if cache == nil {
		t.Fatal("NewGroupMembershipCache returned nil")
	}
	if cache.groups == nil {
		t.Fatal("cache.groups is nil")
	}
	if len(cache.groups) != 0 {
		t.Errorf("expected empty cache, got %d groups", len(cache.groups))
	}
}

func TestIsUserInGroup(t *testing.T) {
	logger := logrus.New()
	cache := NewGroupMembershipCache(logger)

	cache.groups["group1"] = []string{"user1", "user2", "user3"}
	cache.groups["group2"] = []string{"user2", "user4"}
	cache.groups["empty-group"] = []string{}

	tests := []struct {
		name      string
		user      string
		groupName string
		want      bool
	}{
		{
			name:      "user exists in group",
			user:      "user1",
			groupName: "group1",
			want:      true,
		},
		{
			name:      "user exists in multiple groups",
			user:      "user2",
			groupName: "group1",
			want:      true,
		},
		{
			name:      "user not in group",
			user:      "user5",
			groupName: "group1",
			want:      false,
		},
		{
			name:      "group doesn't exist",
			user:      "user1",
			groupName: "nonexistent",
			want:      false,
		},
		{
			name:      "empty group",
			user:      "user1",
			groupName: "empty-group",
			want:      false,
		},
		{
			name:      "empty user string",
			user:      "",
			groupName: "group1",
			want:      false,
		},
		{
			name:      "empty group name",
			user:      "user1",
			groupName: "",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cache.IsUserInGroup(tt.user, tt.groupName)
			if got != tt.want {
				t.Errorf("IsUserInGroup(%q, %q) = %v, want %v", tt.user, tt.groupName, got, tt.want)
			}
		})
	}
}

func TestGetGroupMembers(t *testing.T) {
	logger := logrus.New()
	cache := NewGroupMembershipCache(logger)

	cache.groups["group1"] = []string{"user1", "user2", "user3"}
	cache.groups["empty-group"] = []string{}

	tests := []struct {
		name      string
		groupName string
		want      []string
	}{
		{
			name:      "group exists with members",
			groupName: "group1",
			want:      []string{"user1", "user2", "user3"},
		},
		{
			name:      "empty group",
			groupName: "empty-group",
			want:      []string{},
		},
		{
			name:      "group doesn't exist",
			groupName: "nonexistent",
			want:      nil,
		},
		{
			name:      "empty group name",
			groupName: "",
			want:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cache.GetGroupMembers(tt.groupName)
			if tt.want == nil {
				if got != nil {
					t.Errorf("GetGroupMembers(%q) = %v, want nil", tt.groupName, got)
				}
			} else {
				if len(got) != len(tt.want) {
					t.Errorf("GetGroupMembers(%q) length = %d, want %d", tt.groupName, len(got), len(tt.want))
					return
				}
				for i, v := range tt.want {
					if got[i] != v {
						t.Errorf("GetGroupMembers(%q)[%d] = %q, want %q", tt.groupName, i, got[i], v)
					}
				}
			}
		})
	}
}
