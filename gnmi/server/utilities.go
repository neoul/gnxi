package server

import (
	"fmt"
	"regexp"
	"strings"

	pb "github.com/openconfig/gnmi/proto/gnmi"
)

// gNMI style XPATH string format is different from the XPATH used in YDB.
// The following functions are used for converting the XPATH used in YDB to gNMI Path.
//  - '\' and ']' elapsed characters are not used.
//  e.g.
//   gNMI XPATH: /subscription[id=1]/sensor-path[path=/interfaces/interface[name=1/1\]]
//   YDB XPATH: /subscription[id=1]/sensor-path[path=/interfaces/interface[name=1/1]]

var (
	idPattern = `[a-zA-Z_][a-zA-Z\d\_\-\.]*`
	// YANG identifiers must follow RFC 6020:
	// https://tools.ietf.org/html/rfc6020#section-6.2.
	idRe = regexp.MustCompile(`^` + idPattern + `$`)
	// The sting representation of List key value pairs must follow the
	// following pattern: [key=value], where key is the List key leaf name,
	// and value is the string representation of key leaf value.
	kvRe = regexp.MustCompile(`^\[` +
		// Key leaf name must be a valid YANG identifier.
		idPattern + `=` +
		// Key leaf value must be a non-empty string, which may contain
		// newlines. Use (?s) to turn on s flag to match newlines.
		`((?s).+)` +
		`\]$`)
)

// ToGNMIPath parses an xpath string into a gnmi Path struct defined in gnmi
// proto. Path convention can be found in
// https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-path-conventions.md
// https://github.com/openconfig/reference/blob/master/rpc/gnmi/gnmi-path-strings.md
//
// For example, xpath /interfaces/interface[name=Ethernet1/2/3]/state/counters
// will be parsed to:
//
//    elem: <name: "interfaces" >
//    elem: <
//        name: "interface"
//        key: <
//            key: "name"
//            value: "Ethernet1/2/3"
//        >
//    >
//    elem: <name: "state" >
//    elem: <name: "counters" >
func ToGNMIPath(xpath string) (*pb.Path, error) {
	xpathElements, err := ParseStringPath(xpath)
	if err != nil {
		return nil, err
	}
	var pbPathElements []*pb.PathElem
	for _, elem := range xpathElements {
		switch v := elem.(type) {
		case string:
			pbPathElements = append(pbPathElements, &pb.PathElem{Name: v})
		case map[string]string:
			n := len(pbPathElements)
			if n == 0 {
				return nil, fmt.Errorf("missing name before key-value list")
			}
			if pbPathElements[n-1].Key != nil {
				return nil, fmt.Errorf("two subsequent key-value lists")
			}
			pbPathElements[n-1].Key = v
		default:
			return nil, fmt.Errorf("wrong data type: %T", v)
		}
	}
	return &pb.Path{Elem: pbPathElements}, nil
}

// ParseStringPath parses a string path and produces a []interface{} of parsed
// path elements. Path elements in a string path are separated by '/'. Each
// path element can either be a schema node name or a List path element. Schema
// node names must be valid YANG identifiers. A List path element is encoded
// using the following pattern: list-name[key1=value1]...[keyN=valueN]. Each
// List path element generates two parsed path elements: List name and a
// map[string]string containing List key-value pairs with value(s) in string
// representation. A '/' within a List key value pair string, i.e., between a
// pair of '[' and ']', does not serve as a path separator, and is allowed to be
// part of a List key leaf value. For example, given a string path:
//	"/a/list-name[k=v/v]/c",
// this API returns:
//	[]interface{}{"a", "list-name", map[string]string{"k": "v/v"}, "c"}.
//
// String path parsing consists of two passes. In the first pass, the input
// string is split into []string using valid separator '/'. An incomplete List
// key value string, i.e, a '[' which starts a List key value string without a
// closing ']', in input string generates an error. In the above example, this
// pass produces:
//	[]string{"a", "list-name[k=v/v]", "c"}.
// In the second pass, each element in split []string is parsed checking syntax
// and pattern correctness. Errors are generated for invalid YANG identifiers,
// malformed List key-value string, etc.. In the above example, the second pass
// produces:
//	[]interface{}{"a", "list-name", map[string]string{"k", "v/v"}, "c"}.
func ParseStringPath(stringPath string) ([]interface{}, error) {
	elems, err := splitPath(stringPath)
	if err != nil {
		return nil, err
	}

	var path []interface{}
	// Check whether each path element is valid. Parse List key value
	// pairs.
	for _, elem := range elems {
		parts, err := parseElement(elem)
		if err != nil {
			return nil, fmt.Errorf("invalid string path %s: %v", stringPath, err)
		}
		path = append(path, parts...)
	}

	return path, nil
}

// parseKeyValueString parses a List key-value pair, and returns a
// map[string]string whose key is the List key leaf name and whose value is the
// string representation of List key leaf value. The input path-valur pairs are
// encoded using the following pattern: [k1=v1][k2=v2]..., where k1 and k2 must be
// valid YANG identifiers, v1 and v2 can be any non-empty strings where any ']'
// must be escapced by an '\'. Any malformed key-value pair generates an error.
// For example, given
//	"[k1=v1][k2=v2]",
// this API returns
//	map[string]string{"k1": "v1", "k2": "v2"}.
func parseKeyValueString(str string) (map[string]string, error) {
	keyValuePairs := make(map[string]string)
	// begin marks the beginning of a key-value pair.
	begin := 0
	// end marks the end of a key-value pair.
	end := 0
	// insideBrackets is true when at least one '[' has been found and no
	// ']' has been found. It is false when a closing ']' has been found.
	insideBrackets := 0

	for end < len(str) {
		switch str[end] {
		case '[':
			if end == 0 || str[end-1] != '\\' {
				insideBrackets++
			}
			end++
		case ']':
			if end == 0 || str[end-1] != '\\' {
				insideBrackets--
				if insideBrackets <= 0 {
					keyValue := str[begin : end+1]
					// Key-value pair string must have the
					// following pattern: [k=v], where k is a valid
					// YANG identifier, and v can be any non-empty
					// string.
					if !kvRe.MatchString(keyValue) {
						return nil, fmt.Errorf("malformed List key-value pair string: %s, in: %s", keyValue, str)
					}
					keyValue = keyValue[1 : len(keyValue)-1]
					i := strings.Index(keyValue, "=")
					key, val := keyValue[:i], keyValue[i+1:]
					// Recover escaped '[' and ']'.
					val = strings.Replace(val, `\]`, `]`, -1)
					val = strings.Replace(val, `\[`, `[`, -1)
					keyValuePairs[key] = val
					begin = end + 1
				}
			}
			end++
		default:
			end++
		}
	}

	if begin < end {
		return nil, fmt.Errorf("malformed List key-value pair string: %s", str)
	}

	return keyValuePairs, nil
}

// parseElement parses a split path element, and returns the parsed elements.
// Two types of path elements are supported:
//
// 1. Non-List schema node names which must be valid YANG identifiers. A valid
// schema node name is returned as it is. For example, given "abc", this API
// returns []interface{"abc"}.
//
// 2. List elements following this pattern: list-name[k1=v1], where list-name
// is the substring from the beginning of the input string to the first '[', k1
// is the substring from the letter after '[' to the first '=', and v1 is the
// substring from the letter after '=' to the first unescaped ']'. list-name
// and k1 must be valid YANG identifier, and v1 can be any non-empty string
// where ']' is escaped by '\'. A List element is parsed into two parts: List
// name and List key value pair(s). List key value pairs are saved in a
// map[string]string whose key is List key leaf name and whose value is the
// string representation of List key leaf value. For example, given
//	"list-name[k1=v1]",
// this API returns
//	[]interface{}{"list-name", map[string]string{"k1": "v1"}}.
// Multi-key List elements follow a similar pattern:
//	list-name[k1=v1]...[kN=vN].
func parseElement(elem string) ([]interface{}, error) {
	i := strings.Index(elem, "[")
	if i < 0 {
		if !idRe.MatchString(elem) {
			return nil, fmt.Errorf("invalid node name: %q", elem)
		}
		return []interface{}{elem}, nil
	}

	listName := elem[:i]
	if !idRe.MatchString(listName) {
		return nil, fmt.Errorf("invalid List name: %q, in: %s", listName, elem)
	}
	keyValuePairs, err := parseKeyValueString(elem[i:])
	if err != nil {
		return nil, fmt.Errorf("invalid path element %s: %v", elem, err)
	}
	return []interface{}{listName, keyValuePairs}, nil
}

// splits XPATH used in YDB.
func splitPath(str string) ([]string, error) {
	var path []string
	str += "/"
	// insideBrackets is true when at least one '[' has been found and no
	// ']' has been found. It is false when a closing ']' has been found.
	insideBrackets := 0
	// begin marks the beginning of a path element, which is separated by
	// '/' unclosed between '[' and ']'.
	begin := 0
	// end marks the end of a path element, which is separated by '/'
	// unclosed between '[' and ']'.
	end := 0

	// Split the given string using unescaped '/'.
	for end < len(str) {
		switch str[end] {
		case '/':
			if insideBrackets <= 0 {
				// Current '/' is a valid path element
				// separator.
				if end > begin {
					path = append(path, str[begin:end])
				}
				end++
				begin = end
			} else {
				// Current '/' must be part of a List key value
				// string.
				end++
			}
		case '[':
			if end == 0 || str[end-1] != '\\' {
				// Current '[' is unescacped, and is the
				// beginning of List key-value pair(s) string.
				insideBrackets++
			}
			end++
		case ']':
			if end == 0 || str[end-1] != '\\' {
				// Current ']' is unescacped, and is the end of
				// List key-value pair(s) string.
				insideBrackets--
			}
			end++
		default:
			end++
		}
	}

	if insideBrackets > 0 {
		return nil, fmt.Errorf("missing ] in path string: %s", str)
	}
	return path, nil
}
