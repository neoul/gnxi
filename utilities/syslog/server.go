package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/neoul/libydb/go/ydb"
	"gopkg.in/mcuadros/go-syslog.v2"
	"gopkg.in/mcuadros/go-syslog.v2/format"
)

func main() {
	channel := make(syslog.LogPartsChannel)
	handler := syslog.NewChannelHandler(channel)

	server := syslog.NewServer()
	// server.SetFormat(syslog.RFC5424)
	// server.SetFormat(syslog.RFC3164)
	server.SetFormat(syslog.Automatic)
	server.SetHandler(handler)
	server.ListenUDP("0.0.0.0:1514")
	server.ListenTCP("0.0.0.0:1514")

	server.Boot()

	db, dbclose := ydb.Open("subsystem")
	defer dbclose()
	err := db.Connect("uss://openconfig", "pub")
	if err != nil {
		log.Println(err)
		return
	}
	db.Serve()

	go func(channel syslog.LogPartsChannel) {
		for logParts := range channel {
			// fmt.Println(logParts)
			// utilities.PrintStruct(logParts)
			// fmt.Println()
			SendMessage(db, logParts)
		}
	}(channel)

	server.Wait()
}

const (
	formatMessage = `
messages:
  state:
    message:
      msg: "%s"
      priority: %d
      app-name: %s
      msgid: %s
`
)

// SendMessage - sends a syslog message via YDB
func SendMessage(db *ydb.YDB, logData format.LogParts) {
	msg := strings.ReplaceAll(logData["content"].(string), `"`, `\"`)
	s := fmt.Sprintf(formatMessage,
		msg,
		logData["priority"],
		logData["tag"], "")
	db.Write([]byte(s))
}

// Syslog headers available for RFC 3164 and RFC 5424
// Syslog RFC 3164 >>	Syslog RFC 5424
// PRIORITY >> PRIORITY
// not matched >> VERSION
// TIMESTAMP >> TIMESTAMP
// HOSTNAME >> HOSTNAME
// not matched >> APP-NAME
// not matched >> PROCID
// not matched >> MSGID
// STRUCTURED-DATA
// MSG >>	MSG

// format.LogParts(
// 	• priority:٭int(&30)
// 	• facility:٭int(&3)
// 	• tls_peer:٭string()
// 	• tag:٭string(&dbus-daemon)
// 	• hostname:٭string(&neoul-note)
// 	• content:٭string(&[system] Successfully activated service 'org.freedesktop.hostname1')
// 	• severity:٭int(&6)
// 	• client:٭string(&127.0.0.1:54272)
// 	• timestamp:٭time.Time(
// 	• • wall:0
// 	• • ext:63733345713
// 	• • loc:<nil>))

// 	format.LogParts(
// 	• timestamp:٭time.Time(
// 	• • wall:0
// 	• • ext:63733345713
// 	• • loc:<nil>)
// 	• content:٭string(&Started Hostname Service.)
// 	• severity:٭int(&6)
// 	• tls_peer:٭string()
// 	• hostname:٭string(&neoul-note)
// 	• tag:٭string(&systemd)
// 	• priority:٭int(&30)
// 	• facility:٭int(&3)
// 	• client:٭string(&127.0.0.1:54272))

// 	format.LogParts(
// 	• timestamp:٭time.Time(
// 	• • wall:0
// 	• • ext:63733345743
// 	• • loc:<nil>)
// 	• tag:٭string(&systemd)
// 	• priority:٭int(&30)
// 	• facility:٭int(&3)
// 	• severity:٭int(&6)
// 	• client:٭string(&127.0.0.1:54272)
// 	• hostname:٭string(&neoul-note)
// 	• content:٭string(&systemd-hostnamed.service: Succeeded.)
// 	• tls_peer:٭string())
