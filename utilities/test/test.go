package test

import (
	"io/ioutil"
	"os"
	"reflect"
	"regexp"
	"strings"
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

// LoadProtoMessages loads ref type's proto message
func LoadProtoMessages(file string) ([]string, error) {
	reg, err := regexp.Compile("##@")
	if err != nil {
		return nil, err
	}
	fp, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer fp.Close()
	data, err := ioutil.ReadAll(fp)
	if err != nil {
		return nil, err
	}

	var start, end int
	allindex := reg.FindAllIndex(data, len(data))
	textlist := make([]string, 0, len(allindex)+1)
	for i := 1; i < len(allindex); i++ {
		start = allindex[i-1][0]
		end = allindex[i][0]
		textlist = append(textlist, strings.Trim(string(data[start:end]), " \n\t"))
	}
	textlist = append(textlist, strings.Trim(string(data[end:]), " \n\t"))
	return textlist, nil
}
