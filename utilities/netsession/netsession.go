package netsession

import (
	"net"
	"sync"

	"github.com/golang/glog"
)

var (
	mutex sync.Mutex

	listeners map[string]*NetListener
	conns     map[string]*NetConn
)

func init() {
	listeners = map[string]*NetListener{}
	conns = map[string]*NetConn{}
}

// NetConn - Wrapping structure for net.Conn.
type NetConn struct {
	net.Conn
	session Session
}

// Close - net.Close wrapping function
func (c NetConn) Close() error {
	mutex.Lock()
	delete(conns, c.RemoteAddr().String())
	mutex.Unlock()
	if c.session != nil {
		c.session.Closed(c.LocalAddr(), c.RemoteAddr())
	}
	if glog.V(5) {
		glog.Info("Closed conn:", c.RemoteAddr().String())
	}
	return c.Conn.Close()
}

// NetListener - Wrapping structure for connected client check.
type NetListener struct {
	net.Listener
	session Session
}

// Listen - creates new NetListener
func Listen(network string, address string, session Session) (*NetListener, error) {
	listener, err := net.Listen(network, address)
	if err != nil {
		return nil, err
	}
	netlistener := &NetListener{
		Listener: listener,
		session:  session,
	}
	mutex.Lock()
	listeners[listener.Addr().String()] = netlistener
	mutex.Unlock()
	return netlistener, err
}

// Accept - net.Accept wrapping function
func (l *NetListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return c, err
	}
	if glog.V(5) {
		glog.Info("Accepted conn:", c.RemoteAddr().String())
	}
	mutex.Lock()
	conn := NetConn{Conn: c, session: l.session}
	conns[conn.RemoteAddr().String()] = &conn
	mutex.Unlock()
	if l.session != nil {
		l.session.Started(conn.LocalAddr(), conn.RemoteAddr())
	}
	return conn, err
}

// Close - net.Close wrapping function
func (l *NetListener) Close() error {
	if glog.V(5) {
		glog.Info("Close all conns")
	}
	mutex.Lock()
	delete(listeners, l.Addr().String())
	mutex.Unlock()
	return l.Listener.Close()
}

// Session Started and Closed interfaces -
// These interface will be invoked if registered on which
// a connection is started or terminated.
type Session interface {
	Started(local, remote net.Addr)
	Closed(local, remote net.Addr)
}

type emptySession struct {
}

func (e *emptySession) Started() {
}

func (e *emptySession) Closed() {
}
