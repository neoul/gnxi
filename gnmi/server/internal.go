package server

import (
	"fmt"

	"github.com/neoul/gnxi/utilities/xpath"
)

const dynamicTeleSubInfoFormat = `
telemetry-system:
 subscriptions:
  dynamic-subscriptions:
   dynamic-subscription[id=%d]:
    id: %d
    state:
     id: %d
     destination-address: %s
     destination-port: %d
     sample-interval: %d
     heartbeat-interval: %d
     suppress-redundant: %v
     protocol: %s
     encoding: ENC_%s
    sensor-paths:`

const dynamicTeleSubInfoPathFormat = `
     sensor-path[path=%s]:
      state:
       path: %s`

func (s *Server) addDynamicTeleInfo(telesubs []*TelemetrySubscription) {
	data := ""
	for _, telesub := range telesubs {
		data += fmt.Sprintf(dynamicTeleSubInfoFormat,
			telesub.ID, telesub.ID, telesub.ID,
			telesub.session.Address,
			telesub.session.Port,
			telesub.Configured.SampleInterval,
			telesub.Configured.HeartbeatInterval,
			telesub.Configured.SuppressRedundant,
			"STREAM_GRPC",
			telesub.Encoding,
		)
		telesub.mutex.Lock()
		for i := range telesub.Paths {
			p := xpath.ToXPath(telesub.Paths[i])
			data += fmt.Sprintf(dynamicTeleSubInfoPathFormat, p, p)
		}
		telesub.mutex.Unlock()
	}
	if data != "" {
		s.idb.Write([]byte(data))
	}
}

func (s *Server) deleteDynamicTeleInfo(teleses *TelemetrySession) {
	data := ""
	for _, telesub := range teleses.Telesub {
		data += fmt.Sprintf(`
telemetry-system:
 subscriptions:
  dynamic-subscriptions:
   dynamic-subscription[id=%d]:
`, telesub.ID)
	}
	if data != "" {
		s.idb.Delete([]byte(data))
	}
}
