package server

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"time"

	"github.com/go-kit/kit/metrics/graphite"
	"github.com/op/go-logging"
)

var (
	log *logging.Logger
)

// Server
type Server struct {
	ConfigListen  string
	ConfigServers []string
	ReadTimeout   time.Duration

	Log             *logging.Logger
	udpConn         *net.UDPConn
	tcpListener     *net.TCPListener
	Channel         chan<- *bytes.Buffer
	Stats           *graphite.Graphite
	statsTCPBytes   *graphite.Counter
	statsUDPBytes   *graphite.Counter
	statsTCPCounter *graphite.Counter
	statsUDPCounter *graphite.Counter
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

	s.statsTCPBytes = s.Stats.NewCounter("incoming.tcpBytes")
	s.statsUDPBytes = s.Stats.NewCounter("incoming.udpBytes")
	s.statsTCPCounter = s.Stats.NewCounter("incoming.tcpCounter")
	s.statsUDPCounter = s.Stats.NewCounter("incoming.udpCounter")

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

	// buf := make([]byte, getSockBufferMaxSize())
	reader := bufio.NewReader(s.udpConn)
	tp := textproto.NewReader(reader)

	go func() error {
		defer s.udpConn.Close()
		for {
			line, err := tp.ReadLineBytes()
			// n, _, err := s.udpConn.ReadFromUDP(buf)
			if err != nil {
				log.Errorf("UDP Server Error: %v", err)
				return err
			}
			n := len(line)
			log.Debugf("UDP Received line %d bytes", n)
			if n > 0 {
				go func() {
					buf, err := s.validate(line)
					if err != nil {
						log.Warningf("UDP %v", err)
						return
					}
					s.Channel <- buf
					s.statsUDPBytes.Add(float64(n))
					s.statsUDPCounter.Add(1)
				}()
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
			conn, err := s.tcpListener.AcceptTCP()
			if err != nil {
				log.Debug("TCP Fail accept with err:", err)
				continue
			}
			log.Debugf("TCP Success accept from %v", conn.RemoteAddr())
			go s.handleTCP(conn)
		}
	}()
	return nil
}

func (s *Server) handleTCP(conn *net.TCPConn) error {
	defer conn.Close()
	// conn.SetDeadline(time.Now().Add(s.ReadTimeout))
	reader := bufio.NewReader(conn)
	tp := textproto.NewReader(reader)
	for {
		line, err := tp.ReadLineBytes()
		n := len(line)
		if err != nil {
			if err == io.EOF {
				log.Debugf("TCP Close connection from %v", conn.RemoteAddr())
				return nil
			}
			log.Debugf("TCP Close connection from %v with err: %v", conn.RemoteAddr(), err)
			return err
		}
		log.Debugf("TCP Received %d bytes from %v", n, conn.RemoteAddr())
		if n > 0 {
			go func() {
				buf, err := s.validate(line)
				if err != nil {
					log.Warningf("TCP %v from %v", err, conn.RemoteAddr())
					return
				}
				s.Channel <- buf
				s.statsTCPBytes.Add(float64(n))
				s.statsTCPCounter.Add(1)
			}()
		}
	}
}

// Reload config
func (s *Server) Reload() error {

	return nil
}

// Stop server
func (s *Server) Stop() error {
	return nil
}

func (s *Server) validate(line []byte) (*bytes.Buffer, error) {
	// Fast validate statsd metrics from heka
	colonPos := bytes.IndexByte(line, ':')
	if colonPos == -1 {
		return nil, fmt.Errorf("Failed to parse line: %s", string(line))
	}
	pipePos := bytes.IndexByte(line, '|')
	if pipePos == -1 {
		return nil, fmt.Errorf("Failed to parse line: %s", string(line))
	}
	// bucket := line[:colonPos]
	// value := line[colonPos+1 : pipePos]

	modifier := line[pipePos+1:]
	lm := len(modifier)
	if lm != 1 && lm != 2 {
		return nil, fmt.Errorf("Failed to parse line: %s", string(line))
	}
	return bytes.NewBuffer(line), nil
}
