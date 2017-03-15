package upstreams

import (
	"bytes"
	"fmt"
	"net"
	"time"

	"github.com/op/go-logging"
)

var (
	log *logging.Logger
)

type backend struct {
	conn    net.Conn
	server  string
	inteval time.Duration
	timeout time.Duration
	retries int
}

type Upstream struct {
	backends      []*backend
	activeBackend int
	Log           *logging.Logger
	Channel       <-chan *bytes.Buffer

	BackendsList             []string
	BackendReconnectInterval int64
	BackendTimeout           int64
	BackendRetries           int
}

func (u *Upstream) Start() {
	log = u.Log
	u.backends = make([]*backend, len(u.BackendsList))
	for i, server := range u.BackendsList {
		newBackend := &backend{
			server:  server,
			inteval: time.Millisecond * time.Duration(u.BackendReconnectInterval),
			timeout: time.Millisecond * time.Duration(u.BackendTimeout),
			retries: u.BackendRetries,
		}
		u.backends[i] = newBackend

		if err := newBackend.Connect(); err != nil {
			log.Errorf("Connect to %s fail with error: %v", server, err)
		} else {
			log.Infof("Connect to %s successfully", server)
		}
	}
	go u.sheduller()
}

func (u *Upstream) Stop() {
	for _, b := range u.backends {
		b.Stop()
	}
}

func (u *Upstream) sheduller() {
	for data := range u.Channel {
		for _, b := range u.backends {
			err := b.SendData(data.Bytes())
			if err == nil {
				break
			}
			log.Errorf("Switch to next backend [%v]", err)
		}
	}
}

func (b *backend) Connect() error {
	var (
		addr *net.TCPAddr
		err  error
	)
	if addr, err = net.ResolveTCPAddr("tcp", b.server); err != nil {
		return err
	}
	if b.conn, err = net.DialTimeout("tcp", addr.String(), b.timeout); err != nil {
		return err
	}
	return nil
}


func (b *backend) SendData(data []byte) error {
	if b.conn != nil {
		if _, err := b.conn.Write(data); err == nil {
			// log.Debugf("Sent %d bytes to %s", n, b.server)
			return nil
		}
	}
	for i := 1; i < b.retries; i++ {
		time.Sleep(b.inteval)
		if err := b.Connect(); err == nil {
			if _, err := b.conn.Write(data); err == nil {
				// log.Debugf("Sent %d bytes to %s", n, b.server)
				return nil
			}
		}
		// log.Debug("Failed reconnect to %s after %d tryes", b.server, i)
	}
	return fmt.Errorf("Can't send data to %s after %d retries", b.server, b.retries)
}

func (b *backend) Stop() {
	b.conn.Close()
}
