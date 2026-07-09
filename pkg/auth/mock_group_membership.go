package auth

// MockGroupMembershipProvider is a mock implementation of GroupMembershipProvider for testing.
type MockGroupMembershipProvider struct {
	GetGroupMembersFn func(groupName string) []string
	Groups            map[string][]string
}

func (m *MockGroupMembershipProvider) GetGroupMembers(groupName string) []string {
	if m.GetGroupMembersFn != nil {
		return m.GetGroupMembersFn(groupName)
	}
	return m.Groups[groupName]
}
