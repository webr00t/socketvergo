package main

import (
	"io"
	"log"
	"net"
)

//Connector is a client side abstraction, it uses the connection for protocol
// and return a transport connection.
type Connector interface {
	Connect(conn net.Conn, network string, addr string) (net.Conn, error)
}

//Dialer is a client side abstraction, it likes connector but creates the connection itself.
type Dialer interface {
	// Dial connects to the address on the named network.
	Dial(network string, addr string) (net.Conn, error)
}

//Handler is a server side abstraction, it accepts the connection and works for protocol.
type Handler interface {
	Handle(conn net.Conn)
}

// Listener is a network listener. It use handler to deal with request.
type Listener struct {
	listener net.Listener
	handler  Handler
	network  string
	addr     string
	done     chan bool
}

// NewListener returns a Listener which listens at addr with network, and uses the handler to handle the request.
func NewListener(network string, addr string, handler Handler) *Listener {
	listener, err := net.Listen(network, addr)
	if err != nil {
		log.Printf("[listener] %s %s: listen failed: %s", network, addr, err)
	}
	return &Listener{
		listener: listener,
		handler:  handler,
		network:  network,
		addr:     addr,
	}
}

// Serve accepts requests in a go routine.
func (s *Listener) Serve() {
	go func() {
	out:
		for {
			select {
			case <-s.done:
				break out
			default:
				conn, err := s.listener.Accept()
				if err != nil {
					log.Printf("[listener] %s %s: bad request", s.network, s.addr)
					continue
				}
				go func() {
					s.handler.Handle(conn)
				}()
			}
		}

		log.Printf("[listener] %s %s: stoped", s.network, s.addr)
	}()
}

func (s *Listener) Stop() {
	s.done <- true
}

type tunnelDialer struct {
	Network   string
	Addr      string
	Connector Connector
}

// NewDialer returns a TunnelDialer.
func NewDialer(network string, addr string, connector Connector) Dialer {
	return &tunnelDialer{
		network,
		addr,
		connector,
	}
}

func (c *tunnelDialer) Dial(network string, addr string) (net.Conn, error) {
	conn, err := net.Dial(c.Network, c.Addr)
	if err != nil {
		return conn, err
	}
	return c.Connector.Connect(conn, network, addr)
}

func transport(rw1, rw2 io.ReadWriter) error {
	errc := make(chan error, 1)
	go func() {
		errc <- copyBuffer(rw1, rw2)
	}()

	go func() {
		errc <- copyBuffer(rw2, rw1)
	}()

	err := <-errc
	if err != nil && err == io.EOF {
		err = nil
	}
	return err
}

func copyBuffer(dst io.Writer, src io.Reader) error {
	buf := LPool.Get().([]byte)
	defer LPool.Put(buf)

	_, err := io.CopyBuffer(dst, src, buf)
	return err
}
