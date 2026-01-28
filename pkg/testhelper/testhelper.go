package testhelper

import (
	"reflect"

	"github.com/google/go-cmp/cmp"
)

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
