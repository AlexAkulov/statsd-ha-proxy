package upstreams

import (
	"bytes"
	"net"
	"time"

	"github.com/go-kit/kit/metrics/graphite"
	"github.com/op/go-logging"
	"fmt"
)

var (
	log *logging.Logger
)

type backend struct {
	statsActive *graphite.Counter
	statsSentBytes *graphite.Counter

	conn     net.Conn
	server   string
	timeout  time.Duration
	uptime   int64
	downtime int64
}

type Upstream struct {
	backends      []*backend
	activeBackend *backend
	Log           *logging.Logger
	Stats         *graphite.Graphite
	Channel       <-chan *bytes.Buffer

	// Эта настройка должна предотварить переключение трафика во время кратковременных сетевых неполадок.
	// Переключение трафика произойдёт после того, как мастер будет недоступен больше заданного, этой настройкой, времени.
	// Так же, возврат трафика на более приоритетный сервер, произойдёт не раньше, чем время соединение с сервером превысит время заданное этой настрйокой
	SwitchLatency time.Duration

	BackendsList             []string
	BackendReconnectInterval time.Duration
	BackendTimeout           time.Duration
}

func (u *Upstream) Start() {
	log = u.Log
	u.backends = make([]*backend, len(u.BackendsList))
	for i := len(u.BackendsList) - 1; i > -1; i-- {
		newBackend := &backend{
			statsSentBytes: u.Stats.NewCounter(fmt.Sprintf("upstrems.%s.sendBytes", u.BackendsList[i])),
			server:   u.BackendsList[i],
			timeout:  u.BackendTimeout,
			downtime: time.Now().Unix(),
			uptime:   time.Now().Unix(),
		}
		u.backends[i] = newBackend

		if err := newBackend.Connect(); err != nil {
			log.Errorf("Connect to %s fail with error: %v", u.BackendsList[i], err)
		} else {
			log.Infof("Connect to %s successfully", u.BackendsList[i])
			u.activeBackend = u.backends[i]
		}
	}
	if u.activeBackend == nil {
		log.Error("No avaliable active backends")
	} else {
		log.Infof("Active backend is %s", u.activeBackend.server)
	}
	go u.watchDog()
	go u.sendData()
}

func (u *Upstream) Stop() error {
	for _, b := range u.backends {
		b.Stop()
	}
	return nil
}

func (u *Upstream) sendData() {
	for buf := range u.Channel {
		for {
			if u.activeBackend.conn != nil {
				n, err := u.activeBackend.conn.Write(buf.Bytes())
				if err == nil {
					u.activeBackend.statsSentBytes.Add(float64(n))
					break
				}
				log.Infof("%s is disconnected", u.activeBackend.server)
				u.activeBackend.conn = nil
				u.activeBackend.downtime = time.Now().Unix()
			}
			time.Sleep(u.SwitchLatency)
		}
	}
}

func (u *Upstream) watchDog() {
	for {
		time.Sleep(u.BackendReconnectInterval)
		priority := len(u.backends)
		for i, backend := range u.backends {
			if err := backend.Connect(); err == nil {
				log.Debugf("%s OK", backend.server)
				if i < priority {
					priority = i
				}
			} else {
				log.Debugf("%s Fail with error: %v", backend.server, err)
			}
		}
		if priority == len(u.backends) {
			log.Errorf("All backends down")
			continue
		}
		if u.activeBackend == nil {
			u.activeBackend = u.backends[priority]
			log.Infof("Now active backend is %s", u.activeBackend.server)
			continue
		}
		if u.activeBackend.server != u.backends[priority].server {
			log.Infof("Switch backend from %s to %s", u.activeBackend.server, u.backends[priority].server)
			u.activeBackend = u.backends[priority]
		}
	}
}

func (b *backend) Connect() error {
	var (
		addr *net.TCPAddr
		err  error
	)
	if b.conn == nil {
		if addr, err = net.ResolveTCPAddr("tcp", b.server); err != nil {
			return err
		}
		if b.conn, err = net.DialTimeout("tcp", addr.String(), b.timeout); err != nil {
			return err
		}
		b.uptime = time.Now().Unix()
	}
	return nil
}

func (b *backend) Stop() {
	if b.conn != nil {
		b.conn.Close()
	}
}
