package tunnel

import (
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/NextChapterSoftware/chissl/share/cio"
	"github.com/NextChapterSoftware/chissl/share/settings"
	"github.com/jpillora/sizestr"
	"golang.org/x/crypto/ssh"
)

func (t *Tunnel) handleSSHRequests(reqs <-chan *ssh.Request) {
	for r := range reqs {
		switch r.Type {
		case "ping":
			_ = r.Reply(true, []byte("pong"))
		default:
			t.Debugf("Unknown request: %s", r.Type)
		}
	}
}

func (t *Tunnel) handleSSHChannels(chans <-chan ssh.NewChannel) {
	for ch := range chans {
		go t.handleSSHChannel(ch)
	}
}

func (t *Tunnel) handleSSHChannel(ch ssh.NewChannel) {
	if !t.Config.Outbound {
		t.Debugf("Denied outbound connection")
		_ = ch.Reject(ssh.Prohibited, "Denied outbound connection")
		return
	}
	remote := string(ch.ExtraData())
	//extract protocol
	hostPort, _ := settings.L4Proto(remote)
	//udp := proto == "udp"
	socks := hostPort == "socks"
	if socks && t.socksServer == nil {
		t.Debugf("Denied socks request, please enable socks")
		_ = ch.Reject(ssh.Prohibited, "SOCKS5 is not enabled")
		return
	}
	sshChan, reqs, err := ch.Accept()
	if err != nil {
		t.Debugf("Failed to accept stream: %s", err)
		return
	}
	stream := io.ReadWriteCloser(sshChan)
	//cnet.MeterRWC(t.Logger.Fork("sshchan"), sshChan)
	defer stream.Close()
	go ssh.DiscardRequests(reqs)
	l := t.Logger.Fork("conn#%d", t.connStats.New())
	//ready to handle
	t.connStats.Open()
	l.Debugf("Open %s", t.connStats.String())
	// Handle TCP connection
	err = t.handleTCP(l, stream, hostPort)
	t.connStats.Close()
	errmsg := ""
	if err != nil && !strings.HasSuffix(err.Error(), "EOF") {
		errmsg = fmt.Sprintf(" (error %s)", err)
	}
	l.Debugf("Close %s%s", t.connStats.String(), errmsg)
}

func (t *Tunnel) handleTCP(l *cio.Logger, src io.ReadWriteCloser, hostPort string) error {
	dst, err := net.Dial("tcp", hostPort)
	if err != nil {
		return err
	}
	var s, r int64
	if l.IsDebug() {
		srcLogger := cio.NewLoggingReadWriteCloser(src, l, fmt.Sprintf("Host: %s ", hostPort))
		defer srcLogger.Close()
		s, r = cio.LoggingPipe(srcLogger, dst)
	} else {
		s, r = cio.Pipe(src, dst)
	}

	l.Debugf("sent %s received %s", sizestr.ToString(s), sizestr.ToString(r))
	return nil
}
