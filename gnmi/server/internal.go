package server

import (
	"fmt"

	"github.com/neoul/gnxi/gnmi/model"
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

func (s *Server) addStreamDynamicSubscription(subs []*Subscription) {
	data := ""
	for _, sub := range subs {
		data += fmt.Sprintf(dynamicTeleSubInfoFormat,
			sub.ID, sub.ID, sub.ID,
			sub.session.Address,
			sub.session.Port,
			sub.Configured.SampleInterval,
			sub.Configured.HeartbeatInterval,
			sub.Configured.SuppressRedundant,
			"STREAM_GRPC",
			sub.Encoding,
		)
		sub.mutex.Lock()
		for i := range sub.Paths {
			p := xpath.ToXPath(sub.Paths[i])
			data += fmt.Sprintf(dynamicTeleSubInfoPathFormat, p, p)
		}
		sub.mutex.Unlock()
	}
	if data != "" {
		s.iStateUpdate.Write([]byte(data))
	}
}

func (s *Server) deleteStreamDynamicSubscriptionInfo(sub *Subscription) {
	data := fmt.Sprintf(`
telemetry-system:
 subscriptions:
  dynamic-subscriptions:
   dynamic-subscription[id=%d]:
`, sub.ID)
	s.iStateUpdate.Delete([]byte(data))
}

func (s *Server) addPollDynamicSubscription(pollsub *PollSubscription) error {
	data := fmt.Sprintf(dynamicTeleSubInfoFormat,
		pollsub.ID, pollsub.ID, pollsub.ID,
		pollsub.session.Address,
		pollsub.session.Port,
		0,
		0,
		false,
		"STREAM_GRPC",
		pollsub.SubList.Encoding,
	)
	s.iStateUpdate.Write([]byte(data))
	return nil
}

func (s *Server) deletePollDynamicSubscriptionInfo(pollsub *PollSubscription) {
	data := fmt.Sprintf(`
telemetry-system:
 subscriptions:
  dynamic-subscriptions:
   dynamic-subscription[id=%d]:
`, pollsub.ID)
	s.iStateUpdate.Delete([]byte(data))
}

// GetInternalStateUpdate returns internal StateUpdate channel.
func (s *Server) GetInternalStateUpdate() model.StateUpdate {
	return s.iStateUpdate
}

// type mempprof struct {
// 	filename string
// 	index    int
// }

// func newMempprof() *mempprof {
// 	return &mempprof{}
// }

// func (m *mempprof) write(file string) {
// 	if m.filename == file {
// 		m.index++
// 	} else {
// 		m.filename = file
// 		m.index = 0
// 	}
// 	f, err := os.Create(fmt.Sprintf("%s.%d", m.filename, m.index))
// 	if err != nil {
// 		log.Fatal("could not create memory profile: ", err)
// 	}
// 	defer f.Close() // error handling omitted for example
// 	runtime.GC()    // get up-to-date statisticsd
// 	if err := pprof.WriteHeapProfile(f); err != nil {
// 		log.Fatal("could not write memory profile: ", err)
// 	}
// }

// var memp *mempprof

// func init() {
// 	memp = newMempprof()
// }
