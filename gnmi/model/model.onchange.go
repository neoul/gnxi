package gostruct

// OnChange - used to inform the data change to user
type OnChange interface {
	OnChange(keys []string, key string, tag string, value string)
}

// OnChange - the data of the device is changed
func (device *Device) OnChange(keys []string, key string, tag string, value string) error {
	return nil
}
