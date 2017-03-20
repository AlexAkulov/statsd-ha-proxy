package server

import (
	"bytes"
	"io"
	"io/ioutil"
	"net"
	"strconv"
	"time"

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
	ReadTimeout   time.Duration

	Log         *logging.Logger
	udpConn     *net.UDPConn
	tcpListener *net.TCPListener
	Channel     chan<- *bytes.Buffer
	tomb        tomb.Tomb
}

// Start server
func (s *Server) Start() error {
	log = s.Log
	if err := s.startUDP(); err != nil {
		return err
	}
	if err := s.startTCP(); err != nil {
		return err
	}

	return nil
}

func (s *Server) startUDP() error {
	var (
		addr *net.UDPAddr
		err  error
	)
	if addr, err = net.ResolveUDPAddr("udp", s.ConfigListen); err != nil {
		return err
	}
	if s.udpConn, err = net.ListenUDP("udp", addr); err != nil {
		return err
	}

	buf := make([]byte, getSockBufferMaxSize())

	go func() error {
		defer s.udpConn.Close()
		for {
			select {
			case <-s.tomb.Dying():
				return nil
			default:
				n, _, err := s.udpConn.ReadFromUDP(buf)
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

func (s *Server) startTCP() error {
	var (
		addr *net.TCPAddr
		err  error
	)
	if addr, err = net.ResolveTCPAddr("tcp", s.ConfigListen); err != nil {
		return err
	}
	if s.tcpListener, err = net.ListenTCP("tcp", addr); err != nil {
		return err
	}

	go func() error {
		defer s.tcpListener.Close()
		for {
			select {
			case <-s.tomb.Dying():
				return nil
			default:
				conn, err := s.tcpListener.AcceptTCP()
				if err != nil {
					log.Error(err)
					continue
				}
				go s.handleTCP(conn)
			}
		}
	}()
	return nil
}

func (s *Server) handleTCP(conn *net.TCPConn) error {
	defer conn.Close()
	buf := make([]byte, getSockBufferMaxSize())
	// conn.SetDeadline(time.Now().Add(s.ReadTimeout))
	for {
		n, err := conn.Read(buf)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			log.Error(err)
			return err
		}
		if n > 0 {
			s.Channel <- bytes.NewBuffer(buf[:n])
		}
	}
}

// Reload config
func (s *Server) Reload() error {

	return nil
}

// Stop server
func (s *Server) Stop() error {
	// tomb.Stop()
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
