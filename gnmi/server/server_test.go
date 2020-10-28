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
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	pb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/value"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/neoul/gnxi/gnmi/model"
	"github.com/neoul/gnxi/gnmi/model/gostruct"
)

var (
	// model is the model for test config server.
	mo = model.NewCustomModel(
		gostruct.Schema,
		gostruct.ΓModelData,
	)
)

func TestCapabilities(t *testing.T) {
	flag.Set("disable-ydb", "true")
	s, err := NewServer(mo, nil, false, false)
	if err != nil {
		t.Fatalf("error in creating server: %v", err)
	}
	defer s.Close()
	resp, err := s.Capabilities(nil, &pb.CapabilityRequest{})
	t.Log(resp)
	if err != nil {
		t.Fatalf("got error %v, want nil", err)
	}
	if !reflect.DeepEqual(resp.GetSupportedModels(), mo.GetModelData()) {
		t.Errorf("got supported models %v\nare not the same as\nmodel supported by the server %v", resp.GetSupportedModels(), mo.GetModelData())
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
	flag.Set("disable-ydb", "true")
	s, err := NewServer(mo, []byte(jsonConfigRoot), true, false)
	if err != nil {
		t.Fatalf("error in creating server: %v", err)
	}
	defer s.Close()

	tds := []struct {
		desc        string
		textPbPath  string
		modelData   []*pb.ModelData
		wantRetCode codes.Code
		wantRespVal interface{}
	}{
		{
			desc: "get valid but non-existing node",
			textPbPath: `
			elem: <name: "system" >
		`,
			wantRetCode: codes.NotFound,
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
			modelData:   []*pb.ModelData{&pb.ModelData{}},
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
func runTestGet(t *testing.T, s *Server, textPbPath string, wantRetCode codes.Code, wantRespVal interface{}, useModels []*pb.ModelData) {
	// Send request
	var pbPath pb.Path
	if err := proto.UnmarshalText(textPbPath, &pbPath); err != nil {
		t.Fatalf("error in unmarshaling path: %v", err)
	}
	req := &pb.GetRequest{
		Path:      []*pb.Path{&pbPath},
		Encoding:  pb.Encoding_JSON_IETF,
		UseModels: useModels,
	}
	t.Log("req:", req)
	resp, err := s.Get(nil, req)
	t.Log("resp:", resp)

	// Check return code
	gotRetStatus, ok := status.FromError(err)
	if !ok {
		t.Fatal("got a non-grpc error from grpc call")
	}
	if gotRetStatus.Code() != wantRetCode {
		t.Fatalf("got return code %v, want %v", gotRetStatus.Code(), wantRetCode)
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

func TestGetWithYdb(t *testing.T) {
	yamlData, err := ioutil.ReadFile("../model/data/sample.yaml")
	if err != nil {
		glog.Exitf("error in reading config file: %v", err)
	}
	flag.Set("disable-ydb", "true")
	s, err := NewServer(mo, yamlData, false, false)
	if err != nil {
		t.Fatalf("error in creating server: %v", err)
	}
	defer s.Close()

	tds := []struct {
		desc        string
		textPbPath  string
		modelData   []*pb.ModelData
		wantRetCode codes.Code
		wantRespVal interface{}
	}{{
		desc: "get valid but non-existing node",
		textPbPath: `
			elem: <name: "system" >
		`,
		wantRetCode: codes.NotFound,
	}, {
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
	}, {
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
	}, {
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
	}, {
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
	}, {
		desc: "non-existing node: wrong path name",
		textPbPath: `
								elem: <name: "interfaces" >
								elem: <
									name: "interface"
									key: <key: "name" value: "eth0" >
								>
								elem: <name: "bar" >`,
		wantRetCode: codes.NotFound,
	}, {
		desc: "non-existing node: wrong path attribute",
		textPbPath: `
								elem: <name: "interfaces" >
								elem: <
									name: "interface"
									key: <key: "foo" value: "eth0" >
								>
								elem: <name: "name" >`,
		wantRetCode: codes.NotFound,
	}, {
		desc:        "use of model data not supported",
		modelData:   []*pb.ModelData{&pb.ModelData{}},
		wantRetCode: codes.Unimplemented,
	}}

	for _, td := range tds {
		t.Run(td.desc, func(t *testing.T) {
			runTestGet(t, s, td.textPbPath, td.wantRetCode, td.wantRespVal, td.modelData)
		})
	}
}

// type gnmiSetTestCase struct {
// 	desc        string                    // description of test case.
// 	initConfig  string                    // config before the operation.
// 	op          pb.UpdateResult_Operation // operation type.
// 	textPbPath  string                    // text format of gnmi Path proto.
// 	val         *pb.TypedValue            // value for UPDATE/REPLACE operations. always nil for DELETE.
// 	wantRetCode codes.Code                // grpc return code.
// 	wantConfig  string                    // config after the operation.
// }

// func TestDelete(t *testing.T) {
// 	tests := []gnmiSetTestCase{{
// 		desc: "delete leaf node",
// 		initConfig: `{
// 			"system": {
// 				"config": {
// 					"hostname": "switch_a",
// 					"login-banner": "Hello!"
// 				}
// 			}
// 		}`,
// 		op: pb.UpdateResult_DELETE,
// 		textPbPath: `
// 			elem: <name: "system" >
// 			elem: <name: "config" >
// 			elem: <name: "login-banner" >
// 		`,
// 		wantRetCode: codes.OK,
// 		wantConfig: `{
// 			"system": {
// 				"config": {
// 					"hostname": "switch_a"
// 				}
// 			}
// 		}`,
// 	}, {
// 		desc: "delete sub-tree",
// 		initConfig: `{
// 			"system": {
// 				"clock": {
// 					"config": {
// 						"timezone-name": "Europe/Stockholm"
// 					}
// 				},
// 				"config": {
// 					"hostname": "switch_a"
// 				}
// 			}
// 		}`,
// 		op: pb.UpdateResult_DELETE,
// 		textPbPath: `
// 			elem: <name: "system" >
// 			elem: <name: "clock" >
// 		`,
// 		wantRetCode: codes.OK,
// 		wantConfig: `{
// 			"system": {
// 				"config": {
// 					"hostname": "switch_a"
// 				}
// 			}
// 		}`,
// 	}, {
// 		desc: "delete a sub-tree with only one leaf node",
// 		initConfig: `{
// 			"system": {
// 				"clock": {
// 					"config": {
// 						"timezone-name": "Europe/Stockholm"
// 					}
// 				},
// 				"config": {
// 					"hostname": "switch_a"
// 				}
// 			}
// 		}`,
// 		op: pb.UpdateResult_DELETE,
// 		textPbPath: `
// 			elem: <name: "system" >
// 			elem: <name: "clock" >
// 			elem: <name: "config" >
// 		`,
// 		wantRetCode: codes.OK,
// 		wantConfig: `{
// 			"system": {
// 				"config": {
// 					"hostname": "switch_a"
// 				}
// 			}
// 		}`,
// 	}, {
// 		desc: "delete a leaf node whose parent has only this child",
// 		initConfig: `{
// 			"system": {
// 				"clock": {
// 					"config": {
// 						"timezone-name": "Europe/Stockholm"
// 					}
// 				},
// 				"config": {
// 					"hostname": "switch_a"
// 				}
// 			}
// 		}`,
// 		op: pb.UpdateResult_DELETE,
// 		textPbPath: `
// 			elem: <name: "system" >
// 			elem: <name: "clock" >
// 			elem: <name: "config" >
// 			elem: <name: "timezone-name" >
// 		`,
// 		wantRetCode: codes.OK,
// 		wantConfig: `{
// 			"system": {
// 				"config": {
// 					"hostname": "switch_a"
// 				}
// 			}
// 		}`,
// 	}, {
// 		desc: "delete root",
// 		initConfig: `{
// 			"system": {
// 				"config": {
// 					"hostname": "switch_a"
// 				}
// 			}
// 		}`,
// 		op:          pb.UpdateResult_DELETE,
// 		wantRetCode: codes.OK,
// 		wantConfig:  `{}`,
// 	}, {
// 		desc: "delete non-existing node",
// 		initConfig: `{
// 			"system": {
// 				"clock": {
// 					"config": {
// 						"timezone-name": "Europe/Stockholm"
// 					}
// 				}
// 			}
// 		}`,
// 		op: pb.UpdateResult_DELETE,
// 		textPbPath: `
// 			elem: <name: "system" >
// 			elem: <name: "clock" >
// 			elem: <name: "config" >
// 			elem: <name: "foo-bar" >
// 		`,
// 		wantRetCode: codes.OK,
// 		wantConfig: `{
// 			"system": {
// 				"clock": {
// 					"config": {
// 						"timezone-name": "Europe/Stockholm"
// 					}
// 				}
// 			}
// 		}`,
// 	}, {
// 		desc: "delete node with non-existing precedent path",
// 		initConfig: `{
// 			"system": {
// 				"clock": {
// 					"config": {
// 						"timezone-name": "Europe/Stockholm"
// 					}
// 				}
// 			}
// 		}`,
// 		op: pb.UpdateResult_DELETE,
// 		textPbPath: `
// 			elem: <name: "system" >
// 			elem: <name: "clock" >
// 			elem: <name: "foo-bar" >
// 			elem: <name: "timezone-name" >
// 		`,
// 		wantRetCode: codes.OK,
// 		wantConfig: `{
// 			"system": {
// 				"clock": {
// 					"config": {
// 						"timezone-name": "Europe/Stockholm"
// 					}
// 				}
// 			}
// 		}`,
// 	}, {
// 		desc: "delete node with non-existing attribute in precedent path",
// 		initConfig: `{
// 			"system": {
// 				"clock": {
// 					"config": {
// 						"timezone-name": "Europe/Stockholm"
// 					}
// 				}
// 			}
// 		}`,
// 		op: pb.UpdateResult_DELETE,
// 		textPbPath: `
// 			elem: <name: "system" >
// 			elem: <name: "clock" >
// 			elem: <
// 				name: "config"
// 				key: <key: "name" value: "foo" >
// 			>
// 			elem: <name: "timezone-name" >`,
// 		wantRetCode: codes.OK,
// 		wantConfig: `{
// 			"system": {
// 				"clock": {
// 					"config": {
// 						"timezone-name": "Europe/Stockholm"
// 					}
// 				}
// 			}
// 		}`,
// 	}, {
// 		desc: "delete node with non-existing attribute",
// 		initConfig: `{
// 			"system": {
// 				"clock": {
// 					"config": {
// 						"timezone-name": "Europe/Stockholm"
// 					}
// 				}
// 			}
// 		}`,
// 		op: pb.UpdateResult_DELETE,
// 		textPbPath: `
// 			elem: <name: "system" >
// 			elem: <name: "clock" >
// 			elem: <name: "config" >
// 			elem: <
// 				name: "timezone-name"
// 				key: <key: "name" value: "foo" >
// 			>
// 			elem: <name: "timezone-name" >`,
// 		wantRetCode: codes.OK,
// 		wantConfig: `{
// 			"system": {
// 				"clock": {
// 					"config": {
// 						"timezone-name": "Europe/Stockholm"
// 					}
// 				}
// 			}
// 		}`,
// 	}, {
// 		desc: "delete leaf node with attribute in its precedent path",
// 		initConfig: `{
// 			"components": {
// 				"component": [
// 					{
// 						"name": "swpri1-1-1",
// 						"config": {
// 							"name": "swpri1-1-1"
// 						},
// 						"state": {
// 							"name": "swpri1-1-1",
// 							"mfg-name": "foo bar inc."
// 						}
// 					}
// 				]
// 			}
// 		}`,
// 		op: pb.UpdateResult_DELETE,
// 		textPbPath: `
// 			elem: <name: "components" >
// 			elem: <
// 				name: "component"
// 				key: <key: "name" value: "swpri1-1-1" >
// 			>
// 			elem: <name: "state" >
// 			elem: <name: "mfg-name" >`,
// 		wantRetCode: codes.OK,
// 		wantConfig: `{
// 			"components": {
// 				"component": [
// 					{
// 						"name": "swpri1-1-1",
// 						"config": {
// 							"name": "swpri1-1-1"
// 						},
// 						"state": {
// 							"name": "swpri1-1-1"
// 						}
// 					}
// 				]
// 			}
// 		}`,
// 	}, {
// 		desc: "delete sub-tree with attribute in its precedent path",
// 		initConfig: `{
// 			"components": {
// 				"component": [
// 					{
// 						"name": "swpri1-1-1",
// 						"config": {
// 							"name": "swpri1-1-1"
// 						},
// 						"state": {
// 							"name": "swpri1-1-1",
// 							"mfg-name": "foo bar inc."
// 						}
// 					}
// 				]
// 			}
// 		}`,
// 		op: pb.UpdateResult_DELETE,
// 		textPbPath: `
// 			elem: <name: "components" >
// 			elem: <
// 				name: "component"
// 				key: <key: "name" value: "swpri1-1-1" >
// 			>
// 			elem: <name: "state" >`,
// 		wantRetCode: codes.OK,
// 		wantConfig: `{
// 			"components": {
// 				"component": [
// 					{
// 						"name": "swpri1-1-1",
// 						"config": {
// 							"name": "swpri1-1-1"
// 						}
// 					}
// 				]
// 			}
// 		}`,
// 	}, {
// 		desc: "delete path node with attribute",
// 		initConfig: `{
// 			"components": {
// 				"component": [
// 					{
// 						"name": "swpri1-1-1",
// 						"config": {
// 							"name": "swpri1-1-1"
// 						}
// 					},
// 					{
// 						"name": "swpri1-1-2",
// 						"config": {
// 							"name": "swpri1-1-2"
// 						}
// 					}
// 				]
// 			}
// 		}`,
// 		op: pb.UpdateResult_DELETE,
// 		textPbPath: `
// 			elem: <name: "components" >
// 			elem: <
// 				name: "component"
// 				key: <key: "name" value: "swpri1-1-1" >
// 			>`,
// 		wantRetCode: codes.OK,
// 		wantConfig: `{
// 			"components": {
// 				"component": [
// 					{
// 						"name": "swpri1-1-2",
// 						"config": {
// 							"name": "swpri1-1-2"
// 						}
// 					}
// 				]
// 			}
// 		}`,
// 	}, {
// 		desc: "delete path node with int type attribute",
// 		initConfig: `{
// 			"system": {
// 				"openflow": {
// 					"controllers": {
// 						"controller": [
// 							{
// 								"config": {
// 									"name": "main"
// 								},
// 								"connections": {
// 									"connection": [
// 										{
// 											"aux-id": 0,
// 											"config": {
// 												"address": "192.0.2.10",
// 												"aux-id": 0
// 											}
// 										}
// 									]
// 								},
// 								"name": "main"
// 							}
// 						]
// 					}
// 				}
// 			}
// 		}`,
// 		op: pb.UpdateResult_DELETE,
// 		textPbPath: `
// 			elem: <name: "system" >
// 			elem: <name: "openflow" >
// 			elem: <name: "controllers" >
// 			elem: <
// 				name: "controller"
// 				key: <key: "name" value: "main" >
// 			>
// 			elem: <name: "connections" >
// 			elem: <
// 				name: "connection"
// 				key: <key: "aux-id" value: "0" >
// 			>
// 			`,
// 		wantRetCode: codes.OK,
// 		wantConfig: `{
// 			"system": {
// 				"openflow": {
// 					"controllers": {
// 						"controller": [
// 							{
// 								"config": {
// 									"name": "main"
// 								},
// 								"name": "main"
// 							}
// 						]
// 					}
// 				}
// 			}
// 		}`,
// 	}, {
// 		desc: "delete leaf node with non-existing attribute value",
// 		initConfig: `{
// 			"components": {
// 				"component": [
// 					{
// 						"name": "swpri1-1-1",
// 						"config": {
// 							"name": "swpri1-1-1"
// 						}
// 					}
// 				]
// 			}
// 		}`,
// 		op: pb.UpdateResult_DELETE,
// 		textPbPath: `
// 			elem: <name: "components" >
// 			elem: <
// 				name: "component"
// 				key: <key: "name" value: "foo" >
// 			>`,
// 		wantRetCode: codes.OK,
// 		wantConfig: `{
// 			"components": {
// 				"component": [
// 					{
// 						"name": "swpri1-1-1",
// 						"config": {
// 							"name": "swpri1-1-1"
// 						}
// 					}
// 				]
// 			}
// 		}`,
// 	}, {
// 		desc: "delete leaf node with non-existing attribute value in precedent path",
// 		initConfig: `{
// 			"components": {
// 				"component": [
// 					{
// 						"name": "swpri1-1-1",
// 						"config": {
// 							"name": "swpri1-1-1"
// 						},
// 						"state": {
// 							"name": "swpri1-1-1",
// 							"mfg-name": "foo bar inc."
// 						}
// 					}
// 				]
// 			}
// 		}`,
// 		op: pb.UpdateResult_DELETE,
// 		textPbPath: `
// 			elem: <name: "components" >
// 			elem: <
// 				name: "component"
// 				key: <key: "name" value: "foo" >
// 			>
// 			elem: <name: "state" >
// 			elem: <name: "mfg-name" >
// 		`,
// 		wantRetCode: codes.OK,
// 		wantConfig: `{
// 			"components": {
// 				"component": [
// 					{
// 						"name": "swpri1-1-1",
// 						"config": {
// 							"name": "swpri1-1-1"
// 						},
// 						"state": {
// 							"name": "swpri1-1-1",
// 							"mfg-name": "foo bar inc."
// 						}
// 					}
// 				]
// 			}
// 		}`,
// 	}}

// 	for _, tc := range tests {
// 		t.Run(tc.desc, func(t *testing.T) {
// 			runTestSet(t, model, tc)
// 		})
// 	}
// }

// func TestReplace(t *testing.T) {
// 	systemConfig := `{
// 		"system": {
// 			"clock": {
// 				"config": {
// 					"timezone-name": "Europe/Stockholm"
// 				}
// 			},
// 			"config": {
// 				"hostname": "switch_a",
// 				"login-banner": "Hello!"
// 			}
// 		}
// 	}`

// 	tests := []gnmiSetTestCase{{
// 		desc:       "replace root",
// 		initConfig: `{}`,
// 		op:         pb.UpdateResult_REPLACE,
// 		val: &pb.TypedValue{
// 			Value: &pb.TypedValue_JsonIetfVal{
// 				JsonIetfVal: []byte(systemConfig),
// 			}},
// 		wantRetCode: codes.OK,
// 		wantConfig:  systemConfig,
// 	}, {
// 		desc:       "replace a subtree",
// 		initConfig: `{}`,
// 		op:         pb.UpdateResult_REPLACE,
// 		textPbPath: `
// 			elem: <name: "system" >
// 			elem: <name: "clock" >
// 		`,
// 		val: &pb.TypedValue{
// 			Value: &pb.TypedValue_JsonIetfVal{
// 				JsonIetfVal: []byte(`{"config": {"timezone-name": "US/New York"}}`),
// 			},
// 		},
// 		wantRetCode: codes.OK,
// 		wantConfig: `{
// 			"system": {
// 				"clock": {
// 					"config": {
// 						"timezone-name": "US/New York"
// 					}
// 				}
// 			}
// 		}`,
// 	}, {
// 		desc:       "replace a keyed list subtree",
// 		initConfig: `{}`,
// 		op:         pb.UpdateResult_REPLACE,
// 		textPbPath: `
// 			elem: <name: "components" >
// 			elem: <
// 				name: "component"
// 				key: <key: "name" value: "swpri1-1-1" >
// 			>`,
// 		val: &pb.TypedValue{
// 			Value: &pb.TypedValue_JsonIetfVal{
// 				JsonIetfVal: []byte(`{"config": {"name": "swpri1-1-1"}}`),
// 			},
// 		},
// 		wantRetCode: codes.OK,
// 		wantConfig: `{
// 			"components": {
// 				"component": [
// 					{
// 						"name": "swpri1-1-1",
// 						"config": {
// 							"name": "swpri1-1-1"
// 						}
// 					}
// 				]
// 			}
// 		}`,
// 	}, {
// 		desc: "replace node with int type attribute in its precedent path",
// 		initConfig: `{
// 			"system": {
// 				"openflow": {
// 					"controllers": {
// 						"controller": [
// 							{
// 								"config": {
// 									"name": "main"
// 								},
// 								"name": "main"
// 							}
// 						]
// 					}
// 				}
// 			}
// 		}`,
// 		op: pb.UpdateResult_REPLACE,
// 		textPbPath: `
// 			elem: <name: "system" >
// 			elem: <name: "openflow" >
// 			elem: <name: "controllers" >
// 			elem: <
// 				name: "controller"
// 				key: <key: "name" value: "main" >
// 			>
// 			elem: <name: "connections" >
// 			elem: <
// 				name: "connection"
// 				key: <key: "aux-id" value: "0" >
// 			>
// 			elem: <name: "config" >
// 		`,
// 		val: &pb.TypedValue{
// 			Value: &pb.TypedValue_JsonIetfVal{
// 				JsonIetfVal: []byte(`{"address": "192.0.2.10", "aux-id": 0}`),
// 			},
// 		},
// 		wantRetCode: codes.OK,
// 		wantConfig: `{
// 			"system": {
// 				"openflow": {
// 					"controllers": {
// 						"controller": [
// 							{
// 								"config": {
// 									"name": "main"
// 								},
// 								"connections": {
// 									"connection": [
// 										{
// 											"aux-id": 0,
// 											"config": {
// 												"address": "192.0.2.10",
// 												"aux-id": 0
// 											}
// 										}
// 									]
// 								},
// 								"name": "main"
// 							}
// 						]
// 					}
// 				}
// 			}
// 		}`,
// 	}, {
// 		desc:       "replace a leaf node of int type",
// 		initConfig: `{}`,
// 		op:         pb.UpdateResult_REPLACE,
// 		textPbPath: `
// 			elem: <name: "system" >
// 			elem: <name: "openflow" >
// 			elem: <name: "agent" >
// 			elem: <name: "config" >
// 			elem: <name: "backoff-interval" >
// 		`,
// 		val: &pb.TypedValue{
// 			Value: &pb.TypedValue_IntVal{IntVal: 5},
// 		},
// 		wantRetCode: codes.OK,
// 		wantConfig: `{
// 			"system": {
// 				"openflow": {
// 					"agent": {
// 						"config": {
// 							"backoff-interval": 5
// 						}
// 					}
// 				}
// 			}
// 		}`,
// 	}, {
// 		desc:       "replace a leaf node of string type",
// 		initConfig: `{}`,
// 		op:         pb.UpdateResult_REPLACE,
// 		textPbPath: `
// 			elem: <name: "system" >
// 			elem: <name: "openflow" >
// 			elem: <name: "agent" >
// 			elem: <name: "config" >
// 			elem: <name: "datapath-id" >
// 		`,
// 		val: &pb.TypedValue{
// 			Value: &pb.TypedValue_StringVal{StringVal: "00:16:3e:00:00:00:00:00"},
// 		},
// 		wantRetCode: codes.OK,
// 		wantConfig: `{
// 			"system": {
// 				"openflow": {
// 					"agent": {
// 						"config": {
// 							"datapath-id": "00:16:3e:00:00:00:00:00"
// 						}
// 					}
// 				}
// 			}
// 		}`,
// 	}, {
// 		desc:       "replace a leaf node of enum type",
// 		initConfig: `{}`,
// 		op:         pb.UpdateResult_REPLACE,
// 		textPbPath: `
// 			elem: <name: "system" >
// 			elem: <name: "openflow" >
// 			elem: <name: "agent" >
// 			elem: <name: "config" >
// 			elem: <name: "failure-mode" >
// 		`,
// 		val: &pb.TypedValue{
// 			Value: &pb.TypedValue_StringVal{StringVal: "SECURE"},
// 		},
// 		wantRetCode: codes.OK,
// 		wantConfig: `{
// 			"system": {
// 				"openflow": {
// 					"agent": {
// 						"config": {
// 							"failure-mode": "SECURE"
// 						}
// 					}
// 				}
// 			}
// 		}`,
// 	}, {
// 		desc:       "replace an non-existing leaf node",
// 		initConfig: `{}`,
// 		op:         pb.UpdateResult_REPLACE,
// 		textPbPath: `
// 			elem: <name: "system" >
// 			elem: <name: "openflow" >
// 			elem: <name: "agent" >
// 			elem: <name: "config" >
// 			elem: <name: "foo-bar" >
// 		`,
// 		val: &pb.TypedValue{
// 			Value: &pb.TypedValue_StringVal{StringVal: "SECURE"},
// 		},
// 		wantRetCode: codes.NotFound,
// 		wantConfig:  `{}`,
// 	}}

// 	for _, tc := range tests {
// 		t.Run(tc.desc, func(t *testing.T) {
// 			runTestSet(t, model, tc)
// 		})
// 	}
// }

// func TestUpdate(t *testing.T) {
// 	tests := []gnmiSetTestCase{{
// 		desc: "update leaf node",
// 		initConfig: `{
// 			"system": {
// 				"config": {
// 					"hostname": "switch_a"
// 				}
// 			}
// 		}`,
// 		op: pb.UpdateResult_UPDATE,
// 		textPbPath: `
// 			elem: <name: "system" >
// 			elem: <name: "config" >
// 			elem: <name: "domain-name" >
// 		`,
// 		val: &pb.TypedValue{
// 			Value: &pb.TypedValue_StringVal{StringVal: "foo.bar.com"},
// 		},
// 		wantRetCode: codes.OK,
// 		wantConfig: `{
// 			"system": {
// 				"config": {
// 					"domain-name": "foo.bar.com",
// 					"hostname": "switch_a"
// 				}
// 			}
// 		}`,
// 	}, {
// 		desc: "update subtree",
// 		initConfig: `{
// 			"system": {
// 				"config": {
// 					"hostname": "switch_a"
// 				}
// 			}
// 		}`,
// 		op: pb.UpdateResult_UPDATE,
// 		textPbPath: `
// 			elem: <name: "system" >
// 			elem: <name: "config" >
// 		`,
// 		val: &pb.TypedValue{
// 			Value: &pb.TypedValue_JsonIetfVal{
// 				JsonIetfVal: []byte(`{"domain-name": "foo.bar.com", "hostname": "switch_a"}`),
// 			},
// 		},
// 		wantRetCode: codes.OK,
// 		wantConfig: `{
// 			"system": {
// 				"config": {
// 					"domain-name": "foo.bar.com",
// 					"hostname": "switch_a"
// 				}
// 			}
// 		}`,
// 	}}

// 	for _, tc := range tests {
// 		t.Run(tc.desc, func(t *testing.T) {
// 			runTestSet(t, model, tc)
// 		})
// 	}
// }

// func runTestSet(t *testing.T, m *Model, tc gnmiSetTestCase) {
// 	// Create a new server with empty config
// 	s, err := NewServer(m, []byte(tc.initConfig), nil)
// 	if err != nil {
// 		t.Fatalf("error in creating config server: %v", err)
// 	}

// 	// Send request
// 	var pbPath pb.Path
// 	if err := proto.UnmarshalText(tc.textPbPath, &pbPath); err != nil {
// 		t.Fatalf("error in unmarshaling path: %v", err)
// 	}
// 	var req *pb.SetRequest
// 	switch tc.op {
// 	case pb.UpdateResult_DELETE:
// 		req = &pb.SetRequest{Delete: []*pb.Path{&pbPath}}
// 	case pb.UpdateResult_REPLACE:
// 		req = &pb.SetRequest{Replace: []*pb.Update{{Path: &pbPath, Val: tc.val}}}
// 	case pb.UpdateResult_UPDATE:
// 		req = &pb.SetRequest{Update: []*pb.Update{{Path: &pbPath, Val: tc.val}}}
// 	default:
// 		t.Fatalf("invalid op type: %v", tc.op)
// 	}
// 	_, err = s.Set(nil, req)

// 	// Check return code
// 	gotRetStatus, ok := status.FromError(err)
// 	if !ok {
// 		t.Fatal("got a non-grpc error from grpc call")
// 	}
// 	if gotRetStatus.Code() != tc.wantRetCode {
// 		t.Fatalf("got return code %v, want %v\nerror message: %v", gotRetStatus.Code(), tc.wantRetCode, err)
// 	}

// 	// Check server config
// 	wantConfigStruct, err := NewModelData(m, []byte(tc.wantConfig))
// 	if err != nil {
// 		t.Fatalf("wantConfig data cannot be loaded as a config struct: %v", err)
// 	}
// 	wantConfigJSON, err := ygot.ConstructIETFJSON(wantConfigStruct, &ygot.RFC7951JSONConfig{})
// 	if err != nil {
// 		t.Fatalf("error in constructing IETF JSON tree from wanted config: %v", err)
// 	}
// 	gotConfigJSON, err := ygot.ConstructIETFJSON(s.config, &ygot.RFC7951JSONConfig{})
// 	if err != nil {
// 		t.Fatalf("error in constructing IETF JSON tree from server config: %v", err)
// 	}
// 	if !reflect.DeepEqual(gotConfigJSON, wantConfigJSON) {
// 		t.Fatalf("got server config %v\nwant: %v", gotConfigJSON, wantConfigJSON)
// 	}
// }
