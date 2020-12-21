/* Copyright 2017 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package server

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	"github.com/neoul/gnxi/utilities/status"
	"github.com/neoul/gnxi/utilities/test"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/value"
	"github.com/openconfig/ygot/ygot"
	"google.golang.org/grpc/codes"
)

func TestCapabilities(t *testing.T) {
	s, err := NewServer(nil)
	if err != nil {
		t.Fatalf("error in creating server: %v", err)
	}
	resp, err := s.Capabilities(nil, &gnmipb.CapabilityRequest{})
	t.Log(resp)
	if err != nil {
		t.Fatalf("got error %v, want nil", err)
	}
	if !reflect.DeepEqual(resp.GetSupportedModels(), s.Model.GetModelData()) {
		t.Errorf("got supported models %v\nare not the same as\nmodel supported by the server %v", resp.GetSupportedModels(), s.Model.GetModelData())
	}
	if !reflect.DeepEqual(resp.GetSupportedEncodings(), supportedEncodings) {
		t.Errorf("got supported encodings %v\nare not the same as\nencodings supported by the server %v", resp.GetSupportedEncodings(), supportedEncodings)
	}
}

func TestGet(t *testing.T) {
	jsonConfigRoot := `{
		"openconfig-messages:messages": {
			"config": {
				"severity": "ERROR"
			},
			"state": {
				"severity": "ERROR",
				"message": {
					"msg" : "Messages presents here.",
					"priority": 10
				}
			}
		},
		"openconfig-interfaces:interfaces": {
			"interface": [
				{
					"name": "p1",
					"config": {
						"name": "p1",
						"type": "iana-if-type:ethernetCsmacd",
						"mtu": 1516,
						"loopback-mode": false,
						"description": "Interface#1",
						"enabled": true
					}
				},
				{
					"name": "p2",
					"config": {
						"name": "p2",
						"type": "iana-if-type:ethernetCsmacd",
						"mtu": 1516,
						"loopback-mode": false,
						"description": "n/a",
						"enabled": true
					}
				}
			]
		}
	}`
	yamlData := `
messages:
  config:
    severity: ERROR
  state:
    severity: ERROR
    message:
      msg: "Messages presents here."
      priority: 10
interfaces:
  interface[name=p1]:
   "name": "p1"
   "config":
      "name": "p1"
      "type": "ethernetCsmacd"
      "mtu": 1516
      "loopback-mode": false
      "description": "Interface#1"
      "enabled": true
  interface[name=p2]:
    "name": "p2"
    "config":
      "name": "p2"
      "type": "ethernetCsmacd"
      "mtu": 1516
      "loopback-mode": false
      "description": "n/a"
      "enabled": true
`

	s, err := NewServer(Startup(yamlData))
	if err != nil {
		t.Fatalf("error in creating server: %v", err)
	}

	tds := []struct {
		desc        string
		textPbPath  string
		modelData   []*gnmipb.ModelData
		wantRetCode codes.Code
		wantRespVal interface{}
	}{
		{
			desc: "get valid but non-existing node",
			textPbPath: `
			elem: <name: "system" >
		`,
			wantRetCode: codes.OK, // [CHECK] codes.NotFound?
		},
		{
			desc:        "root node",
			wantRetCode: codes.OK,
			wantRespVal: jsonConfigRoot,
		},
		{
			desc: "get non-enum type",
			textPbPath: `
					elem: <name: "messages" >
					elem: <name: "state" >
					elem: <name: "message" >
					elem: <name: "priority" >
				`,
			wantRetCode: codes.OK,
			wantRespVal: uint64(10),
		},
		{
			desc: "get enum type",
			textPbPath: `
					elem: <name: "messages" >
					elem: <name: "state" >
					elem: <name: "severity" >
				`,
			wantRetCode: codes.OK,
			wantRespVal: "ERROR",
		},
		{
			desc:        "root child node",
			textPbPath:  `elem: <name: "interfaces" >`,
			wantRetCode: codes.OK,
			wantRespVal: `{
						"openconfig-interfaces:interface": [
							{
								"name": "p1",
								"config": {
									"name": "p1",
									"type": "iana-if-type:ethernetCsmacd",
									"mtu": 1516,
									"loopback-mode": false,
									"description": "Interface#1",
									"enabled": true
								}
							},
							{
								"name": "p2",
								"config": {
									"name": "p2",
									"type": "iana-if-type:ethernetCsmacd",
									"mtu": 1516,
									"loopback-mode": false,
									"description": "n/a",
									"enabled": true
								}
							}
						]
					}`,
		},
		{
			desc: "node with attribute",
			textPbPath: `
								elem: <name: "interfaces" >
								elem: <
									name: "interface"
									key: <key: "name" value: "p1" >
								>`,
			wantRetCode: codes.OK,
			wantRespVal: `{
				"openconfig-interfaces:name": "p1",
				"openconfig-interfaces:config": {
					"name": "p1",
					"type": "iana-if-type:ethernetCsmacd",
					"mtu": 1516,
					"loopback-mode": false,
					"description": "Interface#1",
					"enabled": true
				}
			}`,
		},
		{
			desc: "node with attribute in its parent",
			textPbPath: `
								elem: <name: "interfaces" >
								elem: <
									name: "interface"
									key: <key: "name" value: "p2" >
								>
								elem: <name: "config" >
								elem: <name: "type" >`,
			wantRetCode: codes.OK,
			wantRespVal: `ethernetCsmacd`,
		},
		{
			desc: "ref leaf node",
			textPbPath: `
								elem: <name: "interfaces" >
								elem: <
									name: "interface"
									key: <key: "name" value: "p1" >
								>
								elem: <name: "name" >`,
			wantRetCode: codes.OK,
			wantRespVal: "p1",
		},
		{
			desc: "regular leaf node",
			textPbPath: `
								elem: <name: "interfaces" >
								elem: <
									name: "interface"
									key: <key: "name" value: "p1" >
								>
								elem: <name: "config" >
								elem: <name: "name" >`,
			wantRetCode: codes.OK,
			wantRespVal: "p1",
		},
		{
			desc: "non-existing node: wrong path name",
			textPbPath: `
								elem: <name: "interfaces" >
								elem: <
									name: "interface"
									key: <key: "name" value: "p1" >
								>
								elem: <name: "bar" >`,
			wantRetCode: codes.NotFound,
		},
		{
			desc: "non-existing node: wrong path attribute",
			textPbPath: `
								elem: <name: "interfaces" >
								elem: <
									name: "interface"
									key: <key: "foo" value: "p1" >
								>
								elem: <name: "name" >`,
			wantRetCode: codes.NotFound,
		},
		{
			desc:        "use of model data not supported",
			modelData:   []*gnmipb.ModelData{&gnmipb.ModelData{}},
			wantRetCode: codes.Unimplemented,
		},
	}

	for _, td := range tds {
		t.Run(td.desc, func(t *testing.T) {
			runTestGet(t, s, td.textPbPath, td.wantRetCode, td.wantRespVal, td.modelData)
		})
	}
}

// runTestGet requests a path from the server by Get grpc call, and compares if
// the return code and response value are expected.
func runTestGet(t *testing.T, s *Server, textPbPath string, wantRetCode codes.Code, wantRespVal interface{}, useModels []*gnmipb.ModelData) {
	// Send request
	var pbPath gnmipb.Path
	if err := proto.UnmarshalText(textPbPath, &pbPath); err != nil {
		t.Fatalf("error in unmarshaling path: %v", err)
	}
	req := &gnmipb.GetRequest{
		Path:      []*gnmipb.Path{&pbPath},
		Encoding:  gnmipb.Encoding_JSON_IETF,
		UseModels: useModels,
	}
	t.Log("req:", req)
	resp, err := s.Get(nil, req)
	t.Log("resp:", resp)

	// Check return code
	if status.Code(err) != wantRetCode {
		t.Fatalf("got return code %v, want %v", status.Code(err), wantRetCode)
	}

	// Check response value
	var gotVal interface{}
	if resp != nil {
		notifs := resp.GetNotification()
		if len(notifs) != 1 {
			t.Fatalf("got %d notifications, want 1", len(notifs))
		}
		updates := notifs[0].GetUpdate()
		if len(updates) != 1 {
			t.Fatalf("got %d updates in the notification, want 1", len(updates))
		}
		val := updates[0].GetVal()
		if val == nil {
			return
		}
		if val.GetJsonIetfVal() == nil {
			gotVal, err = value.ToScalar(val)
			if err != nil {
				t.Errorf("got: %v, want a scalar value", gotVal)
			}
		} else {
			// Unmarshal json data to gotVal container for comparison
			if err := json.Unmarshal(val.GetJsonIetfVal(), &gotVal); err != nil {
				t.Fatalf("error in unmarshaling IETF JSON data to json container: %v", err)
			}
			var wantJSONStruct interface{}
			if err := json.Unmarshal([]byte(wantRespVal.(string)), &wantJSONStruct); err != nil {
				t.Fatalf("error in unmarshaling IETF JSON data to json container: %v", err)
			}
			wantRespVal = wantJSONStruct
		}
	}

	if !reflect.DeepEqual(gotVal, wantRespVal) {
		t.Errorf("got: %v (%T),\nwant %v (%T)", gotVal, gotVal, wantRespVal, wantRespVal)
	}
}

func TestGetWithYaml(t *testing.T) {
	yamlData, err := ioutil.ReadFile("../model/data/sample.yaml")
	if err != nil {
		glog.Exitf("error in reading config file: %v", err)
	}

	s, err := NewServer(Startup(yamlData))
	if err != nil {
		t.Fatalf("error in creating server: %v", err)
	}

	tds := []struct {
		desc        string
		textPbPath  string
		modelData   []*gnmipb.ModelData
		wantRetCode codes.Code
		wantRespVal interface{}
	}{
		{
			desc: "get valid but non-existing node",
			textPbPath: `
			elem: <name: "system" >
		`,
			wantRetCode: codes.OK, // [CHECK] codes.NotFound?
		},
		{
			desc: "node with attribute",
			textPbPath: `
								elem: <name: "interfaces" >
								elem: <
									name: "interface"
									key: <key: "name" value: "eth1" >
								>
								elem: <name: "config" >`,
			wantRetCode: codes.OK,
			wantRespVal: `{
				"openconfig-interfaces:name": "eth1",
				"openconfig-interfaces:type": "iana-if-type:ethernetCsmacd",
				"openconfig-interfaces:mtu": 1516,
				"openconfig-interfaces:loopback-mode": false,
				"openconfig-interfaces:description": "ethernet card #2",
				"openconfig-interfaces:enabled": true
			}`,
		},
		{
			desc: "node with attribute in its parent",
			textPbPath: `
								elem: <name: "interfaces" >
								elem: <
									name: "interface"
									key: <key: "name" value: "eth1" >
								>
								elem: <name: "config" >
								elem: <name: "type" >`,
			wantRetCode: codes.OK,
			wantRespVal: `ethernetCsmacd`,
		},
		{
			desc: "ref leaf node",
			textPbPath: `
								elem: <name: "interfaces" >
								elem: <
									name: "interface"
									key: <key: "name" value: "eth0" >
								>
								elem: <name: "name" >`,
			wantRetCode: codes.OK,
			wantRespVal: "eth0",
		},
		{
			desc: "regular leaf node",
			textPbPath: `
								elem: <name: "interfaces" >
								elem: <
									name: "interface"
									key: <key: "name" value: "eth0" >
								>
								elem: <name: "config" >
								elem: <name: "name" >`,
			wantRetCode: codes.OK,
			wantRespVal: "eth0",
		},
		{
			desc: "non-existing node: wrong path name",
			textPbPath: `
								elem: <name: "interfaces" >
								elem: <
									name: "interface"
									key: <key: "name" value: "eth0" >
								>
								elem: <name: "bar" >`,
			wantRetCode: codes.NotFound,
		},
		{
			desc: "non-existing node: wrong path attribute",
			textPbPath: `
								elem: <name: "interfaces" >
								elem: <
									name: "interface"
									key: <key: "foo" value: "eth0" >
								>
								elem: <name: "name" >`,
			wantRetCode: codes.NotFound,
		},
		{
			desc:        "use of model data not supported",
			modelData:   []*gnmipb.ModelData{&gnmipb.ModelData{}},
			wantRetCode: codes.Unimplemented,
		},
	}

	for _, td := range tds {
		t.Run(td.desc, func(t *testing.T) {
			runTestGet(t, s, td.textPbPath, td.wantRetCode, td.wantRespVal, td.modelData)
		})
	}
}

type gnmiSetTestCase struct {
	desc        string                        // description of test case.
	initConfig  string                        // config before the operation.
	op          gnmipb.UpdateResult_Operation // operation type.
	textPbPath  string                        // text format of gnmi Path proto.
	val         *gnmipb.TypedValue            // value for UPDATE/REPLACE operations. always nil for DELETE.
	wantRetCode codes.Code                    // grpc return code.
	wantConfig  string                        // config after the operation.
}

func TestDelete(t *testing.T) {
	tests := []gnmiSetTestCase{{
		desc: "delete leaf node",
		initConfig: `{
			"system": {
				"config": {
					"hostname": "switch_a",
					"login-banner": "Hello!"
				}
			}
		}`,
		op: gnmipb.UpdateResult_DELETE,
		textPbPath: `
			elem: <name: "system" >
			elem: <name: "config" >
			elem: <name: "login-banner" >
		`,
		wantRetCode: codes.OK,
		wantConfig: `{
			"system": {
				"config": {
					"hostname": "switch_a"
				}
			}
		}`,
	}, {
		desc: "delete sub-tree",
		initConfig: `{
			"system": {
				"clock": {
					"config": {
						"timezone-name": "Europe/Stockholm"
					}
				},
				"config": {
					"hostname": "switch_a"
				}
			}
		}`,
		op: gnmipb.UpdateResult_DELETE,
		textPbPath: `
			elem: <name: "system" >
			elem: <name: "clock" >
		`,
		wantRetCode: codes.OK,
		wantConfig: `{
			"system": {
				"config": {
					"hostname": "switch_a"
				}
			}
		}`,
	}, {
		desc: "delete a sub-tree with only one leaf node",
		initConfig: `{
			"system": {
				"clock": {
					"config": {
						"timezone-name": "Europe/Stockholm"
					}
				},
				"config": {
					"hostname": "switch_a"
				}
			}
		}`,
		op: gnmipb.UpdateResult_DELETE,
		textPbPath: `
			elem: <name: "system" >
			elem: <name: "clock" >
			elem: <name: "config" >
		`,
		wantRetCode: codes.OK,
		wantConfig: `{
			"system": {
				"config": {
					"hostname": "switch_a"
				}
			}
		}`,
	}, {
		desc: "delete a leaf node whose parent has only this child",
		initConfig: `{
			"system": {
				"clock": {
					"config": {
						"timezone-name": "Europe/Stockholm"
					}
				},
				"config": {
					"hostname": "switch_a"
				}
			}
		}`,
		op: gnmipb.UpdateResult_DELETE,
		textPbPath: `
			elem: <name: "system" >
			elem: <name: "clock" >
			elem: <name: "config" >
			elem: <name: "timezone-name" >
		`,
		wantRetCode: codes.OK,
		wantConfig: `{
			"system": {
				"config": {
					"hostname": "switch_a"
				}
			}
		}`,
	}, {
		desc: "delete root",
		initConfig: `{
			"system": {
				"config": {
					"hostname": "switch_a"
				}
			}
		}`,
		op:          gnmipb.UpdateResult_DELETE,
		wantRetCode: codes.OK,
		wantConfig:  `{}`,
	}, {
		desc: "delete non-existing node",
		initConfig: `{
			"system": {
				"clock": {
					"config": {
						"timezone-name": "Europe/Stockholm"
					}
				}
			}
		}`,
		op: gnmipb.UpdateResult_DELETE,
		textPbPath: `
			elem: <name: "system" >
			elem: <name: "clock" >
			elem: <name: "config" >
			elem: <name: "foo-bar" >
		`,
		wantRetCode: codes.OK,
		wantConfig: `{
			"system": {
				"clock": {
					"config": {
						"timezone-name": "Europe/Stockholm"
					}
				}
			}
		}`,
	}, {
		desc: "delete node with non-existing precedent path",
		initConfig: `{
			"system": {
				"clock": {
					"config": {
						"timezone-name": "Europe/Stockholm"
					}
				}
			}
		}`,
		op: gnmipb.UpdateResult_DELETE,
		textPbPath: `
			elem: <name: "system" >
			elem: <name: "clock" >
			elem: <name: "foo-bar" >
			elem: <name: "timezone-name" >
		`,
		wantRetCode: codes.OK,
		wantConfig: `{
			"system": {
				"clock": {
					"config": {
						"timezone-name": "Europe/Stockholm"
					}
				}
			}
		}`,
	}, {
		desc: "delete node with non-existing attribute in precedent path",
		initConfig: `{
			"system": {
				"clock": {
					"config": {
						"timezone-name": "Europe/Stockholm"
					}
				}
			}
		}`,
		op: gnmipb.UpdateResult_DELETE,
		textPbPath: `
			elem: <name: "system" >
			elem: <name: "clock" >
			elem: <
				name: "config"
				key: <key: "name" value: "foo" >
			>
			elem: <name: "timezone-name" >`,
		wantRetCode: codes.OK,
		wantConfig: `{
			"system": {
				"clock": {
					"config": {
						"timezone-name": "Europe/Stockholm"
					}
				}
			}
		}`,
	}, {
		desc: "delete node with non-existing attribute",
		initConfig: `{
			"system": {
				"clock": {
					"config": {
						"timezone-name": "Europe/Stockholm"
					}
				}
			}
		}`,
		op: gnmipb.UpdateResult_DELETE,
		textPbPath: `
			elem: <name: "system" >
			elem: <name: "clock" >
			elem: <name: "config" >
			elem: <
				name: "timezone-name"
				key: <key: "name" value: "foo" >
			>
			elem: <name: "timezone-name" >`,
		wantRetCode: codes.OK,
		wantConfig: `{
			"system": {
				"clock": {
					"config": {
						"timezone-name": "Europe/Stockholm"
					}
				}
			}
		}`,
	}, {
		desc: "delete leaf node with attribute in its precedent path",
		initConfig: `{
			"components": {
				"component": [
					{
						"name": "swpri1-1-1",
						"config": {
							"name": "swpri1-1-1"
						},
						"state": {
							"name": "swpri1-1-1",
							"mfg-name": "foo bar inc."
						}
					}
				]
			}
		}`,
		op: gnmipb.UpdateResult_DELETE,
		textPbPath: `
			elem: <name: "components" >
			elem: <
				name: "component"
				key: <key: "name" value: "swpri1-1-1" >
			>
			elem: <name: "state" >
			elem: <name: "mfg-name" >`,
		wantRetCode: codes.OK,
		wantConfig: `{
			"components": {
				"component": [
					{
						"name": "swpri1-1-1",
						"config": {
							"name": "swpri1-1-1"
						},
						"state": {
							"name": "swpri1-1-1"
						}
					}
				]
			}
		}`,
	}, {
		desc: "delete sub-tree with attribute in its precedent path",
		initConfig: `{
			"components": {
				"component": [
					{
						"name": "swpri1-1-1",
						"config": {
							"name": "swpri1-1-1"
						},
						"state": {
							"name": "swpri1-1-1",
							"mfg-name": "foo bar inc."
						}
					}
				]
			}
		}`,
		op: gnmipb.UpdateResult_DELETE,
		textPbPath: `
			elem: <name: "components" >
			elem: <
				name: "component"
				key: <key: "name" value: "swpri1-1-1" >
			>
			elem: <name: "state" >`,
		wantRetCode: codes.OK,
		wantConfig: `{
			"components": {
				"component": [
					{
						"name": "swpri1-1-1",
						"config": {
							"name": "swpri1-1-1"
						}
					}
				]
			}
		}`,
	}, {
		desc: "delete path node with attribute",
		initConfig: `{
			"components": {
				"component": [
					{
						"name": "swpri1-1-1",
						"config": {
							"name": "swpri1-1-1"
						}
					},
					{
						"name": "swpri1-1-2",
						"config": {
							"name": "swpri1-1-2"
						}
					}
				]
			}
		}`,
		op: gnmipb.UpdateResult_DELETE,
		textPbPath: `
			elem: <name: "components" >
			elem: <
				name: "component"
				key: <key: "name" value: "swpri1-1-1" >
			>`,
		wantRetCode: codes.OK,
		wantConfig: `{
			"components": {
				"component": [
					{
						"name": "swpri1-1-2",
						"config": {
							"name": "swpri1-1-2"
						}
					}
				]
			}
		}`,
	}, {
		desc: "delete path node with int type attribute",
		initConfig: `{
			"system": {
				"openflow": {
					"controllers": {
						"controller": [
							{
								"config": {
									"name": "main"
								},
								"connections": {
									"connection": [
										{
											"aux-id": 0,
											"config": {
												"address": "192.0.2.10",
												"aux-id": 0
											}
										}
									]
								},
								"name": "main"
							}
						]
					}
				}
			}
		}`,
		op: gnmipb.UpdateResult_DELETE,
		textPbPath: `
			elem: <name: "system" >
			elem: <name: "openflow" >
			elem: <name: "controllers" >
			elem: <
				name: "controller"
				key: <key: "name" value: "main" >
			>
			elem: <name: "connections" >
			elem: <
				name: "connection"
				key: <key: "aux-id" value: "0" >
			>
			`,
		wantRetCode: codes.OK,
		wantConfig: `{
			"system": {
				"openflow": {
					"controllers": {
						"controller": [
							{
								"config": {
									"name": "main"
								},
								"name": "main"
							}
						]
					}
				}
			}
		}`,
	}, {
		desc: "delete leaf node with non-existing attribute value",
		initConfig: `{
			"components": {
				"component": [
					{
						"name": "swpri1-1-1",
						"config": {
							"name": "swpri1-1-1"
						}
					}
				]
			}
		}`,
		op: gnmipb.UpdateResult_DELETE,
		textPbPath: `
			elem: <name: "components" >
			elem: <
				name: "component"
				key: <key: "name" value: "foo" >
			>`,
		wantRetCode: codes.OK,
		wantConfig: `{
			"components": {
				"component": [
					{
						"name": "swpri1-1-1",
						"config": {
							"name": "swpri1-1-1"
						}
					}
				]
			}
		}`,
	}, {
		desc: "delete leaf node with non-existing attribute value in precedent path",
		initConfig: `{
			"components": {
				"component": [
					{
						"name": "swpri1-1-1",
						"config": {
							"name": "swpri1-1-1"
						},
						"state": {
							"name": "swpri1-1-1",
							"mfg-name": "foo bar inc."
						}
					}
				]
			}
		}`,
		op: gnmipb.UpdateResult_DELETE,
		textPbPath: `
			elem: <name: "components" >
			elem: <
				name: "component"
				key: <key: "name" value: "foo" >
			>
			elem: <name: "state" >
			elem: <name: "mfg-name" >
		`,
		wantRetCode: codes.OK,
		wantConfig: `{
			"components": {
				"component": [
					{
						"name": "swpri1-1-1",
						"config": {
							"name": "swpri1-1-1"
						},
						"state": {
							"name": "swpri1-1-1",
							"mfg-name": "foo bar inc."
						}
					}
				]
			}
		}`,
	}}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			runTestSet(t, tc)
		})
	}
}

func TestReplace(t *testing.T) {
	systemConfig := `{
		"system": {
			"clock": {
				"config": {
					"timezone-name": "Europe/Stockholm"
				}
			},
			"config": {
				"hostname": "switch_a",
				"login-banner": "Hello!"
			}
		}
	}`

	tests := []gnmiSetTestCase{{
		desc:       "replace root",
		initConfig: `{}`,
		op:         gnmipb.UpdateResult_REPLACE,
		val: &gnmipb.TypedValue{
			Value: &gnmipb.TypedValue_JsonIetfVal{
				JsonIetfVal: []byte(systemConfig),
			}},
		wantRetCode: codes.OK,
		wantConfig:  systemConfig,
	}, {
		desc:       "replace a subtree",
		initConfig: `{}`,
		op:         gnmipb.UpdateResult_REPLACE,
		textPbPath: `
			elem: <name: "system" >
			elem: <name: "clock" >
		`,
		val: &gnmipb.TypedValue{
			Value: &gnmipb.TypedValue_JsonIetfVal{
				JsonIetfVal: []byte(`{"config": {"timezone-name": "US/New York"}}`),
			},
		},
		wantRetCode: codes.OK,
		wantConfig: `{
			"system": {
				"clock": {
					"config": {
						"timezone-name": "US/New York"
					}
				}
			}
		}`,
	}, {
		desc:       "replace a keyed list subtree",
		initConfig: `{}`,
		op:         gnmipb.UpdateResult_REPLACE,
		textPbPath: `
			elem: <name: "components" >
			elem: <
				name: "component"
				key: <key: "name" value: "swpri1-1-1" >
			>`,
		val: &gnmipb.TypedValue{
			Value: &gnmipb.TypedValue_JsonIetfVal{
				JsonIetfVal: []byte(`{"config": {"name": "swpri1-1-1"}}`),
			},
		},
		wantRetCode: codes.OK,
		wantConfig: `{
			"components": {
				"component": [
					{
						"name": "swpri1-1-1",
						"config": {
							"name": "swpri1-1-1"
						}
					}
				]
			}
		}`,
	}, {
		desc: "replace node with int type attribute in its precedent path",
		initConfig: `{
			"system": {
				"openflow": {
					"controllers": {
						"controller": [
							{
								"config": {
									"name": "main"
								},
								"name": "main"
							}
						]
					}
				}
			}
		}`,
		op: gnmipb.UpdateResult_REPLACE,
		textPbPath: `
			elem: <name: "system" >
			elem: <name: "openflow" >
			elem: <name: "controllers" >
			elem: <
				name: "controller"
				key: <key: "name" value: "main" >
			>
			elem: <name: "connections" >
			elem: <
				name: "connection"
				key: <key: "aux-id" value: "0" >
			>
			elem: <name: "config" >
		`,
		val: &gnmipb.TypedValue{
			Value: &gnmipb.TypedValue_JsonIetfVal{
				JsonIetfVal: []byte(`{"address": "192.0.2.10", "aux-id": 0}`),
			},
		},
		wantRetCode: codes.OK,
		wantConfig: `{
			"system": {
				"openflow": {
					"controllers": {
						"controller": [
							{
								"config": {
									"name": "main"
								},
								"connections": {
									"connection": [
										{
											"aux-id": 0,
											"config": {
												"address": "192.0.2.10",
												"aux-id": 0
											}
										}
									]
								},
								"name": "main"
							}
						]
					}
				}
			}
		}`,
	}, {
		desc:       "replace a leaf node of int type",
		initConfig: `{}`,
		op:         gnmipb.UpdateResult_REPLACE,
		textPbPath: `
			elem: <name: "system" >
			elem: <name: "openflow" >
			elem: <name: "agent" >
			elem: <name: "config" >
			elem: <name: "backoff-interval" >
		`,
		val: &gnmipb.TypedValue{
			Value: &gnmipb.TypedValue_UintVal{UintVal: 5},
		},
		wantRetCode: codes.OK,
		wantConfig: `{
			"system": {
				"openflow": {
					"agent": {
						"config": {
							"backoff-interval": 5
						}
					}
				}
			}
		}`,
	}, {
		desc:       "replace a leaf node of string type",
		initConfig: `{}`,
		op:         gnmipb.UpdateResult_REPLACE,
		textPbPath: `
			elem: <name: "system" >
			elem: <name: "openflow" >
			elem: <name: "agent" >
			elem: <name: "config" >
			elem: <name: "datapath-id" >
		`,
		val: &gnmipb.TypedValue{
			Value: &gnmipb.TypedValue_StringVal{StringVal: "00:16:3e:00:00:00:00:00"},
		},
		wantRetCode: codes.OK,
		wantConfig: `{
			"system": {
				"openflow": {
					"agent": {
						"config": {
							"datapath-id": "00:16:3e:00:00:00:00:00"
						}
					}
				}
			}
		}`,
	}, {
		desc:       "replace a leaf node of enum type",
		initConfig: `{}`,
		op:         gnmipb.UpdateResult_REPLACE,
		textPbPath: `
			elem: <name: "system" >
			elem: <name: "openflow" >
			elem: <name: "agent" >
			elem: <name: "config" >
			elem: <name: "failure-mode" >
		`,
		val: &gnmipb.TypedValue{
			Value: &gnmipb.TypedValue_StringVal{StringVal: "SECURE"},
		},
		wantRetCode: codes.OK,
		wantConfig: `{
			"system": {
				"openflow": {
					"agent": {
						"config": {
							"failure-mode": "SECURE"
						}
					}
				}
			}
		}`,
	}, {
		desc:       "replace an non-existing leaf node",
		initConfig: `{}`,
		op:         gnmipb.UpdateResult_REPLACE,
		textPbPath: `
			elem: <name: "system" >
			elem: <name: "openflow" >
			elem: <name: "agent" >
			elem: <name: "config" >
			elem: <name: "foo-bar" >
		`,
		val: &gnmipb.TypedValue{
			Value: &gnmipb.TypedValue_StringVal{StringVal: "SECURE"},
		},
		wantRetCode: codes.InvalidArgument,
		wantConfig:  `{}`,
	}}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			runTestSet(t, tc)
		})
	}
}

func TestUpdate(t *testing.T) {
	tests := []gnmiSetTestCase{{
		desc: "update leaf node",
		initConfig: `{
			"system": {
				"config": {
					"hostname": "switch_a"
				}
			}
		}`,
		op: gnmipb.UpdateResult_UPDATE,
		textPbPath: `
			elem: <name: "system" >
			elem: <name: "config" >
			elem: <name: "domain-name" >
		`,
		val: &gnmipb.TypedValue{
			Value: &gnmipb.TypedValue_StringVal{StringVal: "foo.bar.com"},
		},
		wantRetCode: codes.OK,
		wantConfig: `{
			"system": {
				"config": {
					"domain-name": "foo.bar.com",
					"hostname": "switch_a"
				}
			}
		}`,
	}, {
		desc: "update subtree",
		initConfig: `{
			"system": {
				"config": {
					"hostname": "switch_a"
				}
			}
		}`,
		op: gnmipb.UpdateResult_UPDATE,
		textPbPath: `
			elem: <name: "system" >
			elem: <name: "config" >
		`,
		val: &gnmipb.TypedValue{
			Value: &gnmipb.TypedValue_JsonIetfVal{
				JsonIetfVal: []byte(`{"domain-name": "foo.bar.com", "hostname": "switch_a"}`),
			},
		},
		wantRetCode: codes.OK,
		wantConfig: `{
			"system": {
				"config": {
					"domain-name": "foo.bar.com",
					"hostname": "switch_a"
				}
			}
		}`,
	}}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			runTestSet(t, tc)
		})
	}
}

func runTestSet(t *testing.T, tc gnmiSetTestCase) {
	// Create a new server with empty config
	s, err := NewServer(Startup(tc.initConfig))
	if err != nil {
		t.Fatalf("error in creating config server: %v", err)
	}

	// Send request
	var pbPath gnmipb.Path
	if err := proto.UnmarshalText(tc.textPbPath, &pbPath); err != nil {
		t.Fatalf("error in unmarshaling path: %v", err)
	}
	var req *gnmipb.SetRequest
	switch tc.op {
	case gnmipb.UpdateResult_DELETE:
		req = &gnmipb.SetRequest{Delete: []*gnmipb.Path{&pbPath}}
	case gnmipb.UpdateResult_REPLACE:
		req = &gnmipb.SetRequest{Replace: []*gnmipb.Update{{Path: &pbPath, Val: tc.val}}}
	case gnmipb.UpdateResult_UPDATE:
		req = &gnmipb.SetRequest{Update: []*gnmipb.Update{{Path: &pbPath, Val: tc.val}}}
	default:
		t.Fatalf("invalid op type: %v", tc.op)
	}
	_, err = s.Set(nil, req)

	// Check return code
	if status.Code(err) != tc.wantRetCode {
		t.Fatalf("got return code %v, want %v\nerror message: %v", status.Code(err), tc.wantRetCode, err)
	}

	// Check server config
	wantServer, err := NewServer(Startup(tc.wantConfig))
	if err != nil {
		t.Fatalf("wantConfig data cannot be loaded as a config struct: %v", err)
	}
	wantConfigJSON, err := ygot.ConstructIETFJSON(wantServer.Model.GetRoot(), &ygot.RFC7951JSONConfig{})
	if err != nil {
		t.Fatalf("error in constructing IETF JSON tree from wanted config: %v", err)
	}
	gotConfigJSON, err := ygot.ConstructIETFJSON(s.Model.GetRoot(), &ygot.RFC7951JSONConfig{})
	if err != nil {
		t.Fatalf("error in constructing IETF JSON tree from server config: %v", err)
	}
	if !reflect.DeepEqual(gotConfigJSON, wantConfigJSON) {
		t.Fatalf("got server config %v\nwant: %v", gotConfigJSON, wantConfigJSON)
	}
}

func clearNotificationTimestamp(r *gnmipb.SubscribeResponse) {
	update := r.GetUpdate()
	if update == nil {
		return
	}
	update.Timestamp = 0
}

func TestSubscribe(t *testing.T) {
	if f := flag.Lookup("v"); f != nil && f.Value.String() == f.DefValue {
		f.Value.Set("99")
	}
	if f := flag.Lookup("alsologtostderr"); f != nil && f.Value.String() == f.DefValue {
		f.Value.Set("true")
	}
	if f := flag.Lookup("stderrthreshold"); f != nil && f.Value.String() == f.DefValue {
		f.Value.Set("info")
	}
	if f := flag.Lookup("stderrthreshold"); f != nil && f.Value.String() == f.DefValue {
		f.Value.Set("info")
	}

	startup := `{
		"openconfig-messages:messages": {
			"config": {
				"severity": "ERROR"
			},
			"state": {
				"severity": "ERROR",
				"message": {
					"msg" : "Messages presents here.",
					"priority": 10
				}
			}
		},
		"openconfig-interfaces:interfaces": {
			"interface": [
				{
					"name": "1/1",
					"config": {
						"name": "1/1",
						"type": "iana-if-type:ethernetCsmacd",
						"mtu": 1516,
						"loopback-mode": false,
						"description": "Interface#1",
						"enabled": true
					},
					"state": {
						"name": "1/1",
						"type": "iana-if-type:ethernetCsmacd",
						"mtu": 1516,
						"loopback-mode": false,
						"description": "Interface#1",
						"enabled": true,
						"counters": {
							"in-pkts": "100",
							"out-pkts": "100"
						}
					}
				},
				{
					"name": "1/2",
					"config": {
						"name": "1/2",
						"type": "iana-if-type:ethernetCsmacd",
						"mtu": 1516,
						"loopback-mode": false,
						"description": "n/a",
						"enabled": true
					}
				}
			]
		}
	}`
	s, err := NewServer(Startup(startup), Aliases{
		"#1/1": "/interfaces/interface[name=1/1]",
		"#1/2": "/interfaces/interface[name=1/2]",
	})
	if err != nil {
		t.Fatalf("error in creating config server: %v", err)
	}
	subses := &SubSession{
		ID:            1,
		Address:       "127.0.0.1",
		Port:          uint16(11112),
		SubList:       map[string]*Subscription{},
		respchan:      make(chan *gnmipb.SubscribeResponse, 256),
		shutdown:      make(chan struct{}),
		waitgroup:     new(sync.WaitGroup),
		clientAliases: newClientAliases(),
		Server:        s,
	}

	type testsubscribe struct {
		name    string
		msgfile string
	}
	SubscribeResponseValidator := func(t *testing.T, subses *SubSession, tc testsubscribe, wantresp chan *gnmipb.SubscribeResponse) {
		// var ok bool
		var got, want *gnmipb.SubscribeResponse
		waitgroup := subses.waitgroup
		gotresp := subses.respchan
		shutdown := subses.shutdown
		defer waitgroup.Done()
		for {
			select {
			case want = <-wantresp:
				t.Log("want-response:", want)
				select {
				case got = <-gotresp:
					clearNotificationTimestamp(got)
					t.Log("got-response:", got)
					if !proto.Equal(got, want) {
						t.Errorf("different response:\ngot : %v\nwant: %v\n", got, want)
					}
				case <-shutdown:
					t.Errorf("different response:\ngot : %v\nwant: %v\n", got, want)
					return
				}
			case got = <-gotresp:
				clearNotificationTimestamp(got)
				t.Log("got-response:*", got)
				select {
				case want = <-wantresp:
					t.Log("want-response:*", want)
					if !proto.Equal(got, want) {
						t.Errorf("different response:\ngot : %v\nwant: %v\n", got, want)
					}
				case <-shutdown:
					t.Errorf("different response:\ngot : %v\nwant: %v\n", got, nil)
					return
				}
			case <-shutdown:
				return
			}
		}
	}

	tests := []testsubscribe{
		{
			name:    "server-aliases",
			msgfile: "data/serverAliases.prototxt",
		},
	}
	j, _ := s.ExportToJSON(true)
	fmt.Println(string(j))

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wantresp := make(chan *gnmipb.SubscribeResponse, 128)
			txtMessages, err := test.LoadProtoMessages(tc.msgfile)
			if err != nil {
				t.Errorf("loading '%s' got error: %v", tc.msgfile, err)
			}
			subses.waitgroup.Add(1)
			go SubscribeResponseValidator(t, subses, tc, wantresp)
			var rcode codes.Code
			for i := range txtMessages {
				newline := strings.Index(txtMessages[i], "\n")
				if newline < 0 {
					continue
				}
				if pos := strings.Index(txtMessages[i], "Error"); pos >= 0 && pos < newline {
					wantErr := status.EmptyProto()
					if err := proto.UnmarshalText(txtMessages[i], wantErr); err != nil {
						t.Errorf("proto message unmarshaling got error: %v", err)
					}
					if wantErr.Code == int32(rcode) {
						t.Errorf("different response:\ngot : %v\nwant: %v\n", rcode, codes.Code(wantErr.Code).String())
					}
				} else if pos := strings.Index(txtMessages[i], "SubscribeRequest"); pos >= 0 && pos < newline {
					req := &gnmipb.SubscribeRequest{}
					if err := proto.UnmarshalText(txtMessages[i], req); err != nil {
						t.Errorf("proto message unmarshaling got error: %v", err)
					}
					t.Log("request:", req)
					err := subses.processSubscribeRequest(req)
					if estatus := status.FromError(err); estatus != nil {
						// fmt.Println(proto.MarshalTextString(status.ToProto(err)))
						t.Log("got-error:", estatus.Code.String(), uint32(estatus.Code))
						rcode = estatus.Code
					} else {
						rcode = codes.OK
					}
				} else if pos := strings.Index(txtMessages[i], "SubscribeResponse"); pos >= 0 && pos < newline {
					resp := &gnmipb.SubscribeResponse{}
					if err := proto.UnmarshalText(txtMessages[i], resp); err != nil {
						t.Errorf("proto message unmarshaling got error: %v", err)
					}
					wantresp <- resp
				}
			}
			time.Sleep(2 * time.Second)
			subses.shutdown <- struct{}{}
			defer func() { subses.stopSubSession() }()
			// wait for SubscribeResponseValidator completion
		})
	}

}
