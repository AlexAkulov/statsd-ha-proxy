package server

import (
	"bytes"
	"io/ioutil"
	"net"
	"strconv"

	"github.com/op/go-logging"
	"gopkg.in/tomb.v2"
)

var (
	log *logging.Logger
)

// Server
type Server struct {
	ConfigListen  string
	ConfigServers []string
	Log           *logging.Logger
	conn          *net.UDPConn
	Channel       chan<- *bytes.Buffer
	tomb          tomb.Tomb
}

// Start server
func (s *Server) Start() error {
	log = s.Log

	addr, err := net.ResolveUDPAddr("udp", s.ConfigListen)
	if err != nil {
		return err
	}
	s.conn, err = net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	log.Infof("Listen on port %s", s.ConfigListen)

	buf := make([]byte, getSockBufferMaxSize())

	go func() error {
		for {
			select {
			case <-s.tomb.Dying():
				return nil
			default:
				n, _, err := s.conn.ReadFromUDP(buf)
				if err != nil {
					log.Errorf("Server Error: %v", err)
				}
				if n > 0 {
					s.Channel <- bytes.NewBuffer(buf[:n])
				}
			}
		}
	}()

	return nil
}

// Reload config
func (s *Server) Reload() error {

	return nil
}

// Stop server
func (s *Server) Stop() error {
	s.conn.Close()
	return nil
}

// sockBufferMaxSize() returns the maximum size that the UDP receive buffer
// in the kernel can be set to.  In bytes.
func getSockBufferMaxSize() int {
	defaultBufferSize := 32 * 1024
	// XXX: This is Linux-only most likely
	data, err := ioutil.ReadFile("/proc/sys/net/core/rmem_max")
	if err != nil {
		return defaultBufferSize
	}

	data = bytes.TrimRight(data, "\n\r")
	i, err := strconv.Atoi(string(data))
	if err != nil {
		return defaultBufferSize
	}

	return i
}
