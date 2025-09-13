package tunnel

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"

	"github.com/NextChapterSoftware/chissl/share/cio"
	"github.com/NextChapterSoftware/chissl/share/settings"
	"github.com/jpillora/sizestr"
	"golang.org/x/crypto/ssh"
)

// sshTunnel exposes a subset of Tunnel to subtypes
type sshTunnel interface {
	getSSH(ctx context.Context) ssh.Conn
}

// Proxy is the inbound portion of a Tunnel
type Proxy struct {
	*cio.Logger
	sshTun   sshTunnel
	id       int
	count    int
	remote   *settings.Remote
	dialer   net.Dialer
	tcp      *net.TCPListener
	https    net.Listener
	tlsConf  *tls.Config
	mu       sync.Mutex
	isClient bool
}

// NewProxy creates a Proxy
func NewProxy(logger *cio.Logger, sshTun sshTunnel, index int, remote *settings.Remote, tlsConf *tls.Config, isClient bool) (*Proxy, error) {
	id := index + 1
	p := &Proxy{
		Logger:   logger.Fork("proxy#%s", remote.String()),
		sshTun:   sshTun,
		id:       id,
		remote:   remote,
		tlsConf:  tlsConf,
		isClient: isClient,
	}
	return p, p.listen()
}

func (p *Proxy) listen() error {
	remotePort := p.remote.LocalPort
	// If the tunnel is on the client side, we don't care just grab any port!
	// I spent 6 hours of my life on this which I will never get back!
	if p.isClient && p.remote.Reverse {
		remotePort = "0"
	}
	addr, err := net.ResolveTCPAddr("tcp", p.remote.LocalHost+":"+remotePort)
	if err != nil {
		return p.Errorf("resolve: %s", err)
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return p.Errorf("tcp: %s", err)
	}
	p.Infof("Listening")
	p.tcp = l
	return nil
}

// Run enables the proxy and blocks while its active,
// close the proxy by cancelling the context.
func (p *Proxy) Run(ctx context.Context) error {
	if p.tlsConf != nil {
		return p.runHTTPS(ctx)
	}
	return p.runTCP(ctx)
}

func (p *Proxy) runTCP(ctx context.Context) error {
	done := make(chan struct{})
	//implements missing net.ListenContext
	go func() {
		select {
		case <-ctx.Done():
			p.tcp.Close()
		case <-done:
		}
	}()
	for {
		src, err := p.tcp.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				//listener closed
				err = nil
			default:
				p.Infof("Accept error: %s", err)
			}
			close(done)
			return err
		}
		go p.pipeRemote(ctx, src)
	}
}

func (p *Proxy) runHTTPS(ctx context.Context) error {
	p.tlsConf.NextProtos = []string{"http/1.1"}
	p.https = tls.NewListener(p.tcp, p.tlsConf)
	p.Infof("Done setting up certs and listener https listener on %s", p.tcp.Addr().String())

	done := make(chan struct{})
	//implements missing net.ListenContext
	go func() {
		select {
		case <-ctx.Done():
			p.tcp.Close()
		case <-done:
		}
	}()
	for {
		src, err := p.https.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				//listener closed
				err = nil
			default:
				p.Infof("Accept error: %s", err)
			}
			close(done)
			return err
		}
		go p.pipeRemote(ctx, src)
	}
}

func (p *Proxy) pipeRemote(ctx context.Context, src io.ReadWriteCloser) {
	defer src.Close()

	p.mu.Lock()
	p.count++
	cid := p.count
	p.mu.Unlock()

	l := p.Fork("conn#%d", cid)
	l.Debugf("Open")
	sshConn := p.sshTun.getSSH(ctx)
	if sshConn == nil {
		l.Debugf("No remote connection")
		return
	}
	// Prepare optional tap early so we can record failures too
	var tap Tap
	if t, ok := p.sshTun.(*Tunnel); ok && t.Config.TapFactory != nil {
		meta := Meta{Username: t.Config.Username, Remote: *p.remote, ConnID: fmt.Sprintf("%d", cid)}
		tap = t.Config.TapFactory(meta)
		if tap != nil {
			tap.OnOpen()
		}
	}
	// Attempt to open SSH channel for this remote
	dst, reqs, err := sshConn.OpenChannel("chisel", []byte(p.remote.Remote()))
	if err != nil {
		l.Infof("Stream error: %s", err)
		if tap != nil {
			// record failure close with zero bytes
			tap.OnClose(0, 0)
		}
		return
	}
	go ssh.DiscardRequests(reqs)
	// Pipe with tee if tap present
	var sent, received int64
	if tap != nil {
		sent, received = cio.PipeWithTee(src, dst, tap.SrcWriter(), tap.DstWriter())
		tap.OnClose(sent, received)
		l.Debugf("Close (sent %s received %s)", sizestr.ToString(sent), sizestr.ToString(received))
		return
	}
	// Fallback: plain pipe
	sent, received = cio.Pipe(src, dst)
	l.Debugf("Close (sent %s received %s)", sizestr.ToString(sent), sizestr.ToString(received))
}

// DeliverToRemote opens an SSH channel to the given remote and writes the payload bytes, then closes.
func (t *Tunnel) DeliverToRemote(ctx context.Context, r *settings.Remote, payload []byte) error {
	sshConn := t.getSSH(ctx)
	if sshConn == nil {
		return fmt.Errorf("no active ssh connection")
	}
	dst, reqs, err := sshConn.OpenChannel("chisel", []byte(r.Remote()))
	if err != nil {
		return err
	}
	go ssh.DiscardRequests(reqs)
	defer dst.Close()
	_, err = dst.Write(payload)
	return err
}

// DeliverToRemoteWithResponse opens an SSH channel to the given remote, writes the payload,
// half-closes the write side, then reads all response bytes until EOF or context timeout.
func (t *Tunnel) DeliverToRemoteWithResponse(ctx context.Context, r *settings.Remote, payload []byte) ([]byte, error) {
	sshConn := t.getSSH(ctx)
	if sshConn == nil {
		return nil, fmt.Errorf("no active ssh connection")
	}
	dst, reqs, err := sshConn.OpenChannel("chisel", []byte(r.Remote()))
	if err != nil {
		return nil, err
	}
	go ssh.DiscardRequests(reqs)
	// Ensure channel closed on exit
	defer dst.Close()
	if _, err := dst.Write(payload); err != nil {
		return nil, err
	}
	respCh := make(chan []byte, 1)
	errCh := make(chan error, 1)
	go func() {
		// Parse a single HTTP response from the channel without requiring EOF
		br := bufio.NewReader(dst)
		rq := &http.Request{Method: "POST"}
		upResp, e := http.ReadResponse(br, rq)
		if e != nil {
			errCh <- e
			return
		}
		defer upResp.Body.Close()
		body, _ := io.ReadAll(upResp.Body)
		// Reconstruct a complete HTTP/1.1 response
		var out bytes.Buffer
		// ensure we don't accidentally re-chunk
		upResp.Header.Del("Transfer-Encoding")
		upResp.ContentLength = int64(len(body))
		upResp.Body = io.NopCloser(bytes.NewReader(body))
		_ = upResp.Write(&out)
		respCh <- out.Bytes()
	}()
	select {
	case <-ctx.Done():
		_ = dst.Close()
		return nil, ctx.Err()
	case e := <-errCh:
		return nil, e
	case b := <-respCh:
		return b, nil
	}
}
