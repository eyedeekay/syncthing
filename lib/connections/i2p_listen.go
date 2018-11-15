// Copyright (C) 2018 The Syncthing Authors.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package connections

import (
    "crypto/tls"
    //"net"
	"net/url"
	"sync"
	//"time"

	"github.com/eyedeekay/syncthing/lib/config"
    "github.com/syncthing/syncthing/lib/nat"
    "github.com/eyedeekay/sam3"
)

func init() {

}

type i2pListener struct {
	onAddressesChangedNotifier

	uri     *url.URL
    sam     *sam3.SAM
	cfg     *config.Wrapper
	tlsCfg  *tls.Config
	stop    chan struct{}
	conns   chan internalConn
	factory listenerFactory

	err error
	mut sync.RWMutex
}

type i2pListenerFactory struct{}

func (g *i2pListener) maybeGenerateKeys() (*sam3.I2PKeys, error) {
    g.sam, g.err = sam3.NewSAM(g.cfg.Options.I2PSAMPort)
    if err != nil {
        return nil, err
    }
    defer sam.Close()
    keys, err := sam.NewKeys()
    if err != nil {
        return nil, err
    }
    return &keys, nil
}


func (g *i2pListener) Serve() {
	g.mut.Lock()
	g.err = nil
	g.mut.Unlock()


	b32addr, err := net.ResolveTCPAddr(t.uri.Scheme, t.uri.Host)
	if err != nil {
		t.mut.Lock()
		t.err = err
		t.mut.Unlock()
		l.Infoln("Listen (BEP/tcp):", err)
		return
	}

	listener, err := net.ListenTCP(t.uri.Scheme, tcaddr)
	if err != nil {
		t.mut.Lock()
		t.err = err
		t.mut.Unlock()
		l.Infoln("Listen (BEP/tcp):", err)
		return
	}
	defer listener.Close()

	l.Infof("TCP listener (%v) starting", listener.Addr())
	defer l.Infof("TCP listener (%v) shutting down", listener.Addr())

	mapping := t.natService.NewMapping(nat.TCP, tcaddr.IP, tcaddr.Port)
	mapping.OnChanged(func(_ *nat.Mapping, _, _ []nat.Address) {
		t.notifyAddressesChanged(t)
	})
	defer t.natService.RemoveMapping(mapping)

	t.mut.Lock()
	t.mapping = mapping
	t.mut.Unlock()

	acceptFailures := 0
	const maxAcceptFailures = 10

	for {
		listener.SetDeadline(time.Now().Add(time.Second))
		conn, err := listener.Accept()
		select {
		case <-t.stop:
			if err == nil {
				conn.Close()
			}
			t.mut.Lock()
			t.mapping = nil
			t.mut.Unlock()
			return
		default:
		}
		if err != nil {
			if err, ok := err.(*net.OpError); !ok || !err.Timeout() {
				l.Warnln("Listen (BEP/tcp): Accepting connection:", err)

				acceptFailures++
				if acceptFailures > maxAcceptFailures {
					// Return to restart the listener, because something
					// seems permanently damaged.
					return
				}

				// Slightly increased delay for each failure.
				time.Sleep(time.Duration(acceptFailures) * time.Second)
			}
			continue
		}

		acceptFailures = 0
		l.Debugln("Listen (BEP/tcp): connect from", conn.RemoteAddr())

		if err := dialer.SetTCPOptions(conn); err != nil {
			l.Debugln("Listen (BEP/tcp): setting tcp options:", err)
		}

		if tc := t.cfg.Options().TrafficClass; tc != 0 {
			if err := dialer.SetTrafficClass(conn, tc); err != nil {
				l.Debugln("Listen (BEP/tcp): setting traffic class:", err)
			}
		}

		tc := tls.Server(conn, t.tlsCfg)
		if err := tlsTimedHandshake(tc); err != nil {
			l.Infoln("Listen (BEP/tcp): TLS handshake:", err)
			tc.Close()
			continue
		}

		t.conns <- internalConn{tc, connTypeTCPServer, tcpPriority}
	}
}

// LANAddresses always returns nil because these are all i2p addresses.
func (t *i2pListener) LANAddresses() []*url.URL {
	//return []*url.URL{t.uri}
    return nil
}

func (g *i2pListener) Error() error {
	g.mut.RLock()
	err := g.err
	g.mut.RUnlock()
	return err
}

func (t *i2pListener) NATType() string {
	return "unknown"
}

func (g *i2pListener) Factory() listenerFactory {
	return g.factory
}

// NAT and TLS configuration options are ignored in this context
// uri is used to configure a connection to the SAM bridge?
func (f *i2pListenerFactory) New(uri *url.URL, cfg *config.Wrapper, tlsCfg *tls.Config, conns chan internalConn, natService *nat.Service) genericListener {
	return &i2pListener{
		uri:        fixupPort(uri, config.Options.I2PSAMPort),
		cfg:        cfg,
		tlsCfg:     nil, //tlsCfg,
		conns:      conns,
		stop:       make(chan struct{}),
		factory:    f,
	}
}


func (i2pListenerFactory) Valid(_ config.Configuration) error {
	// Always valid for now. Return error on SAM bridge failure or malfunction
	return nil
}
