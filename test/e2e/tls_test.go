package e2e_test

import (
	"path"
	"testing"

	chclient "github.com/NextChapterSoftware/chissl/client"
	chserver "github.com/NextChapterSoftware/chissl/server"
)

func TestMTLS(t *testing.T) {
	tlsConfig, err := newTestTLSConfig()
	if err != nil {
		t.Fatal(err)
	}
	defer tlsConfig.Close()
	//provide no client cert, server should reject the client request
	tlsConfig.serverTLS.CA = path.Dir(tlsConfig.serverTLS.CA)

	tmpPort := availablePort()
	//setup server, client, fileserver
	teardown := simpleSetup(t,
		&chserver.Config{
			Auth: "admin:admin",
			TLS:  *tlsConfig.serverTLS,
		},
		&chclient.Config{
			Remotes: []string{tmpPort + ":127.0.0.1->$FILEPORT"},
			Auth:    "admin:admin",
			TLS:     *tlsConfig.clientTLS,
			Server:  "https://localhost:" + tmpPort,
		})
	defer teardown()
	//test remote
	result, err := postWithTls("https://localhost:"+tmpPort, "foo", tlsConfig)
	if err != nil {
		t.Fatal(err)
	}
	if result != "foo!" {
		t.Fatalf("expected exclamation mark added")
	}
}

func TestTLSMissingClientCert(t *testing.T) {
	tlsConfig, err := newTestTLSConfig()
	if err != nil {
		t.Fatal(err)
	}
	defer tlsConfig.Close()
	//provide no client cert, server should reject the client request
	tlsConfig.clientTLS.Cert = ""
	tlsConfig.clientTLS.Key = ""

	tmpPort := availablePort()
	//setup server, client, fileserver
	teardown := simpleSetup(t,
		&chserver.Config{
			Auth: "admin:admin",
			TLS:  *tlsConfig.serverTLS,
		},
		&chclient.Config{
			Remotes: []string{tmpPort + ":127.0.0.1->$FILEPORT"},
			Auth:    "admin:admin",
			TLS:     *tlsConfig.clientTLS,
			Server:  "https://localhost:" + tmpPort,
		})
	defer teardown()
	//test remote
	_, err = post("http://localhost:"+tmpPort, "foo")
	if err == nil {
		t.Fatal(err)
	}
}

func TestTLSMissingClientCA(t *testing.T) {
	tlsConfig, err := newTestTLSConfig()
	if err != nil {
		t.Fatal(err)
	}
	defer tlsConfig.Close()
	//specify a CA which does not match the client cert
	//server should reject the client request
	//provide no client cert, server should reject the client request
	tlsConfig.serverTLS.CA = tlsConfig.clientTLS.CA

	tmpPort := availablePort()
	//setup server, client, fileserver
	teardown := simpleSetup(t,
		&chserver.Config{
			Auth: "admin:admin",
			TLS:  *tlsConfig.serverTLS,
		},
		&chclient.Config{
			Remotes: []string{tmpPort + ":127.0.0.1->$FILEPORT"},
			Auth:    "admin:admin",
			TLS:     *tlsConfig.clientTLS,
			Server:  "https://localhost:" + tmpPort,
		})
	defer teardown()
	//test remote
	_, err = post("http://localhost:"+tmpPort, "foo")
	if err == nil {
		t.Fatal(err)
	}
}
