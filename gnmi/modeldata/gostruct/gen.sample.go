package gostruct

// Generate m6424 model
//go:generate sh -c "cd $GOPATH/src && go run github.com/openconfig/ygot/generator/generator.go -include_model_data -generate_fakeroot -output_file github.com/neoul/gnxi/gnmi/modeldata/gostruct/generated.go -package_name gostruct -exclude_modules ietf-interfaces -path github.com/neoul/gnxi/gnmi/modeldata/yang github.com/neoul/gnxi/gnmi/modeldata/yang/openconfig-interfaces.yang github.com/neoul/gnxi/gnmi/modeldata/yang/openconfig-messages.yang github.com/neoul/gnxi/gnmi/modeldata/yang/iana-if-type@2017-01-19.yang"
