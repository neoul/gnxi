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

// Package modeldata contains the following model data in gnmi proto struct:
//  - openconfig-interfaces.yang 2.4.3
//  - hfr-oc-dev.yang 0.1.1
//  - hfr-oc-roe.yang 0.1.1
//  - hfr-oc-tsn.yang 0.1.1
package modeldata

import (
	pb "github.com/openconfig/gnmi/proto/gnmi"
)

var (
	// ModelData is a list of supported models.
	ModelData = []*pb.ModelData{{
		Name:         "openconfig-interfaces",
		Organization: "OpenConfig working group",
		Version:      "2.4.3",
	}, {
		Name:         "hfr-oc-dev",
		Organization: "HFR,Inc. for Mobile Internet",
		Version:      "0.1.1",
	}, {
		Name:         "hfr-oc-roe",
		Organization: "HFR,Inc. for Mobile Internet",
		Version:      "0.1.1",
	}, {
		Name:         "hfr-oc-tsn",
		Organization: "HFR,Inc. for Mobile Internet",
		Version:      "0.1.1",
	}}
)
