package test

import (
	"reflect"
)

// isTypeInterface reports whether v is an interface.
func isTypeInterface(t reflect.Type) bool {
	if t == reflect.TypeOf(nil) {
		return false
	}
	return t.Kind() == reflect.Interface
}

// IsEqualList returns d1, d2 interfaces are equal or not.
func IsEqualList(d1, d2 interface{}) bool {
	v1 := reflect.ValueOf(d1)
	v2 := reflect.ValueOf(d2)
	if isTypeInterface(v1.Type()) {
		v1 = v1.Elem()
	}
	if isTypeInterface(v2.Type()) {
		v2 = v2.Elem()
	}

	if v1.Kind() != reflect.Slice && v1.Kind() != v2.Kind() {
		return false
	}

	for v1.Len() != v2.Len() {
		return false
	}

	l := v1.Len()
	for i := 0; i < l; i++ {
		eq := false
		// fmt.Println("v1", v1.Index(i).Interface())
		for j := 0; j < l; j++ {
			// fmt.Println("v2", v2.Index(j).Interface())
			if reflect.DeepEqual(v1.Index(i).Interface(), v2.Index(j).Interface()) {
				eq = true
				break
			}
		}
		if !eq {
			return false
		}
	}
	return true
}
