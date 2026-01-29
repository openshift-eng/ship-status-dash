package testhelper

import (
	"reflect"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"ship-status-dash/pkg/types"

	"github.com/google/go-cmp/cmp"
)

// SetupTestDB creates an in-memory SQLite database for testing and migrates the standard outage-related models.
// The database is automatically closed when the test completes.
func SetupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	err = db.AutoMigrate(&types.Outage{}, &types.Reason{}, &types.SlackThread{})
	if err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			sqlDB.Close()
		}
	})
	return db
}

// EquateErrorMessage reports errors to be equal if both are nil or both have the same message.
var EquateErrorMessage = cmp.FilterValues(func(x, y interface{}) bool {
	_, ok1 := x.(error)
	_, ok2 := y.(error)
	return ok1 && ok2
}, cmp.Comparer(func(x, y interface{}) bool {
	xe := x.(error)
	ye := y.(error)
	if xe == nil || ye == nil {
		return xe == nil && ye == nil
	}
	return xe.Error() == ye.Error()
}))

// EquateNilEmpty treats nil slices and empty slices as equal.
var EquateNilEmpty = cmp.FilterValues(func(x, y interface{}) bool {
	vx := reflect.ValueOf(x)
	vy := reflect.ValueOf(y)
	return (vx.Kind() == reflect.Slice || vx.Kind() == reflect.Array) &&
		(vy.Kind() == reflect.Slice || vy.Kind() == reflect.Array)
}, cmp.Comparer(func(x, y interface{}) bool {
	vx := reflect.ValueOf(x)
	vy := reflect.ValueOf(y)

	// Handle nil cases
	if vx.IsNil() && vy.IsNil() {
		return true
	}
	if vx.IsNil() {
		return vy.Len() == 0
	}
	if vy.IsNil() {
		return vx.Len() == 0
	}

	// Both are non-nil, compare lengths and elements
	if vx.Len() != vy.Len() {
		return false
	}
	for i := 0; i < vx.Len(); i++ {
		if !cmp.Equal(vx.Index(i).Interface(), vy.Index(i).Interface()) {
			return false
		}
	}
	return true
}))
