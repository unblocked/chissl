package capture

import (
	"github.com/NextChapterSoftware/chissl/share/tunnel"
)

// NewTapFactory creates a tunnel TapFactory bound to this capture Service and a tunnel id/user
func NewTapFactory(svc *Service, tunnelID string, username string, maxEvents int) tunnel.TapFactory {
	return func(meta tunnel.Meta) tunnel.Tap {
		return &TapImpl{svc: svc, meta: Meta{Username: meta.Username, Remote: meta.Remote, ConnID: meta.ConnID}, tunnelID: tunnelID, maxEvents: maxEvents}
	}
}
