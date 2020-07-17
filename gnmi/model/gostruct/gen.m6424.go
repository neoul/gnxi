package gostruct

// Generate m6424 model
//go:generate sh -c "cd $GOPATH/src && go run github.com/openconfig/ygot/generator/generator.go -include_model_data -generate_fakeroot -output_file github.com/neoul/gnxi/gnmi/model/gostruct/generated.go -package_name gostruct -exclude_modules ietf-interfaces -path github.com/neoul/gnxi/gnmi/model/yang github.com/neoul/gnxi/gnmi/model/yang/hfr-oc-dev.yang github.com/neoul/gnxi/gnmi/model/yang/hfr-oc-roe.yang github.com/neoul/gnxi/gnmi/model/yang/openconfig-interfaces.yang github.com/neoul/gnxi/gnmi/model/yang/hfr-oc-tsn.yang github.com/neoul/gnxi/gnmi/model/yang/openconfig-messages.yang"
