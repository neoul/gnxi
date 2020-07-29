package utilities

import (
	"flag"
	"reflect"
)

// GetFlag - Get the flag defined in another package.
func GetFlag(name string, defValue interface{}) interface{} {
	f := flag.Lookup(name)
	if f == nil {
		return defValue
	}
	v := reflect.ValueOf(f.Value)
	if v.Kind() == reflect.Interface {
		v = v.Elem()
	}
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Bool:
		return v.Convert(reflect.TypeOf(false)).Interface()
	case reflect.String:
		return v.Convert(reflect.TypeOf("")).Interface()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		var i int
		return v.Convert(reflect.TypeOf(i)).Interface()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		var i uint
		return v.Convert(reflect.TypeOf(i)).Interface()
	case reflect.Float32, reflect.Float64:
		var f float64
		return v.Convert(reflect.TypeOf(f)).Interface()
	}
	return defValue
}
