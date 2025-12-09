package chserver

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"os"
	"os/user"
	"path/filepath"

	"github.com/NextChapterSoftware/chissl/share/settings"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

// TLSConfig enables configures TLS
type TLSConfig struct {
	Key     string
	Cert    string
	Domains []string
	CA      string
}

func (s *Server) listener(host, port string) (net.Listener, error) {
	hasDomains := len(s.config.TLS.Domains) > 0
	hasKeyCert := s.config.TLS.Key != "" && s.config.TLS.Cert != ""
	if hasDomains && hasKeyCert {
		return nil, errors.New("cannot use key/cert and domains")
	}
	var tlsConf *tls.Config
	if hasDomains {
		tlsConf = s.tlsLetsEncrypt(s.config.TLS.Domains)
	}
	extra := ""
	if hasKeyCert {
		c, err := s.tlsKeyCert(s.config.TLS.Key, s.config.TLS.Cert, s.config.TLS.CA)
		if err != nil {
			return nil, err
		}
		tlsConf = c
		if port != "443" && hasDomains {
			extra = " (WARNING: LetsEncrypt will attempt to connect to your domain on port 443)"
		}
	}
	//tcp listen
	l, err := net.Listen("tcp", host+":"+port)
	if err != nil {
		return nil, err
	}
	//optionally wrap in tls
	proto := "http"
	if tlsConf != nil {
		s.config.TlsConf = tlsConf
		proto += "s"
		l = tls.NewListener(l, tlsConf)
	}
	if err == nil {
		s.Infof("Listening on %s://%s:%s%s", proto, host, port, extra)
	}
	return l, nil
}

func (s *Server) tlsLetsEncrypt(domains []string) *tls.Config {
	//prepare cert manager
	m := &autocert.Manager{
		Prompt: func(tosURL string) bool {
			s.Infof("Accepting LetsEncrypt TOS and fetching certificate (TOS: %s)...", tosURL)
			return true
		},
		Email:      settings.Env("LE_EMAIL"),
		HostPolicy: autocert.HostWhitelist(domains...),
	}
	// Use staging environment if CHISEL_LE_STAGING is set (for testing)
	if settings.Env("LE_STAGING") != "" {
		s.Infof("Using Let's Encrypt STAGING environment (certs won't be trusted)")
		m.Client = &acme.Client{
			DirectoryURL: "https://acme-staging-v02.api.letsencrypt.org/directory",
		}
	}
	//configure file cache
	c := settings.Env("LE_CACHE")
	if c == "" {
		h := os.Getenv("HOME")
		if h == "" {
			if u, err := user.Current(); err == nil {
				h = u.HomeDir
			}
		}
		c = filepath.Join(h, ".cache", "chisel")
	}
	if c != "-" {
		s.Infof("LetsEncrypt cache directory %s", c)
		m.Cache = autocert.DirCache(c)
	}

	// Start HTTP-01 challenge listener on port 80 for ACME validation
	go func() {
		s.Infof("Starting HTTP-01 challenge listener on :80 for LetsEncrypt validation")
		// Wrap the handler to log challenge requests
		challengeHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(r.URL.Path) > 28 && r.URL.Path[:28] == "/.well-known/acme-challenge/" {
				s.Infof("ACME challenge request: %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
			}
			m.HTTPHandler(nil).ServeHTTP(w, r)
		})
		httpServer := &http.Server{
			Addr:    ":80",
			Handler: challengeHandler,
		}
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.Infof("HTTP-01 challenge listener error: %v", err)
		}
	}()

	//return lets-encrypt tls config
	return m.TLSConfig()
}

func (s *Server) tlsKeyCert(key, cert string, ca string) (*tls.Config, error) {
	keypair, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		return nil, err
	}
	//file based tls config using tls defaults
	c := &tls.Config{
		Certificates: []tls.Certificate{keypair},
	}
	//mTLS requires server's CA
	if ca != "" {
		if err := addCA(ca, c); err != nil {
			return nil, err
		}
		s.Infof("Loaded CA path: %s", ca)
	}
	return c, nil
}

func addCA(ca string, c *tls.Config) error {
	fileInfo, err := os.Stat(ca)
	if err != nil {
		return err
	}
	clientCAPool := x509.NewCertPool()
	if fileInfo.IsDir() {
		//this is a directory holding CA bundle files
		files, err := os.ReadDir(ca)
		if err != nil {
			return err
		}
		//add all cert files from path
		for _, file := range files {
			f := file.Name()
			if err := addPEMFile(filepath.Join(ca, f), clientCAPool); err != nil {
				return err
			}
		}
	} else {
		//this is a CA bundle file
		if err := addPEMFile(ca, clientCAPool); err != nil {
			return err
		}
	}
	//set client CAs and enable cert verification
	c.ClientCAs = clientCAPool
	c.ClientAuth = tls.RequireAndVerifyClientCert
	return nil
}

func addPEMFile(path string, pool *x509.CertPool) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !pool.AppendCertsFromPEM(content) {
		return errors.New("Fail to load certificates from : " + path)
	}
	return nil
}

// prewarmTLSCert triggers certificate fetching by making a test TLS connection
// This ensures the Let's Encrypt certificate is fetched before other listeners start
func (s *Server) prewarmTLSCert() error {
	if s.config.TlsConf == nil || s.config.TlsConf.GetCertificate == nil {
		// No dynamic certificate (not Let's Encrypt), nothing to pre-warm
		return nil
	}

	// Use the first domain to trigger certificate fetch
	if len(s.config.TLS.Domains) == 0 {
		return nil
	}
	domain := s.config.TLS.Domains[0]

	// Trigger GetCertificate by calling it directly with a ClientHelloInfo
	hello := &tls.ClientHelloInfo{
		ServerName: domain,
	}
	cert, err := s.config.TlsConf.GetCertificate(hello)
	if err != nil {
		return err
	}
	if cert == nil {
		return errors.New("no certificate returned")
	}
	return nil
}
