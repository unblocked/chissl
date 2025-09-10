package capture

import (
	"fmt"
	"github.com/NextChapterSoftware/chissl/share/tunnel"
	"io"
)

// NewTapFactory creates a tunnel TapFactory bound to this capture Service and a tunnel id/user
func NewTapFactory(svc *Service, tunnelID string, username string, maxEvents int) tunnel.TapFactory {
	return func(meta tunnel.Meta) tunnel.Tap {
		return &TapImpl{svc: svc, meta: Meta{Username: meta.Username, Remote: meta.Remote, ConnID: meta.ConnID}, tunnelID: tunnelID, maxEvents: maxEvents}
	}
}

// NewPerRemoteTapFactory creates a TapFactory which assigns events to per-remote IDs (baseID-r{index})
// portToIndex maps remote.LocalPort -> index used in the ID suffix
func NewPerRemoteTapFactory(svc *Service, baseTunnelID string, username string, maxEvents int, portToIndex map[string]int) tunnel.TapFactory {
	return func(meta tunnel.Meta) tunnel.Tap {
		idx := 0
		if portToIndex != nil {
			if v, ok := portToIndex[meta.Remote.LocalPort]; ok {
				idx = v
			}
		}
		id := fmt.Sprintf("%s-r%d", baseTunnelID, idx)
		return NewTapFactory(svc, id, username, maxEvents)(meta)
	}
}

// dualTap duplicates lifecycle and byte streams to two underlying taps
type dualTap struct{ a, b tunnel.Tap }

func (d dualTap) OnOpen() {
	if d.a != nil {
		d.a.OnOpen()
	}
	if d.b != nil {
		d.b.OnOpen()
	}
}
func (d dualTap) OnClose(s, r int64) {
	if d.a != nil {
		d.a.OnClose(s, r)
	}
	if d.b != nil {
		d.b.OnClose(s, r)
	}
}
func (d dualTap) SrcWriter() io.Writer { return io.MultiWriter(d.a.SrcWriter(), d.b.SrcWriter()) }
func (d dualTap) DstWriter() io.Writer { return io.MultiWriter(d.a.DstWriter(), d.b.DstWriter()) }

// NewDualTapFactory emits capture to both base session ID and per-remote IDs
func NewDualTapFactory(svc *Service, baseTunnelID string, username string, maxEvents int, portToIndex map[string]int) tunnel.TapFactory {
	perRemote := NewPerRemoteTapFactory(svc, baseTunnelID, username, maxEvents, portToIndex)
	base := NewTapFactory(svc, baseTunnelID, username, maxEvents)
	return func(meta tunnel.Meta) tunnel.Tap {
		return dualTap{a: base(meta), b: perRemote(meta)}
	}
}

// NewTripleTapFactoryWithCanonical emits capture to:
// - base session ID (sess-...)
// - per-remote session IDs (sess-...-rN)
// - canonical per-user+ports tunnel ID (tun-<b64user>-<lp>-<rp>)
func NewTripleTapFactoryWithCanonical(svc *Service, baseTunnelID string, username string, maxEvents int, portToIndex map[string]int, unameEnc string) tunnel.TapFactory {
	perRemote := NewPerRemoteTapFactory(svc, baseTunnelID, username, maxEvents, portToIndex)
	base := NewTapFactory(svc, baseTunnelID, username, maxEvents)
	return func(meta tunnel.Meta) tunnel.Tap {
		// Canonical ID derived from remote ports
		lp := meta.Remote.LocalPort
		rp := meta.Remote.RemotePort
		canonicalID := fmt.Sprintf("tun-%s-%s-%s", unameEnc, lp, rp)
		canonical := NewTapFactory(svc, canonicalID, username, maxEvents)
		return dualTap{a: base(meta), b: dualTap{a: perRemote(meta), b: canonical(meta)}}
	}
}
