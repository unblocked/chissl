package tunnel

import (
	"io"

	"github.com/NextChapterSoftware/chissl/share/settings"
)

// Meta describes the context of a tapped connection
// within a tunnel owned by a user.
type Meta struct {
	Username string
	Remote   settings.Remote
	ConnID   string
}

// Tap receives lifecycle and byte-stream callbacks for a single connection.
type Tap interface {
	OnOpen()
	SrcWriter() io.Writer // bytes flowing from src->dst (client -> upstream)
	DstWriter() io.Writer // bytes flowing from dst->src (upstream -> client)
	OnClose(sent int64, received int64)
}

// TapFactory creates a Tap for a given connection meta. It can
// return nil to disable capture for that connection.
type TapFactory func(meta Meta) Tap
