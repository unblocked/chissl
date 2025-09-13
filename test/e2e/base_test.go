package e2e_test

import (
	chclient "github.com/NextChapterSoftware/chissl/client"
	chserver "github.com/NextChapterSoftware/chissl/server"
	"testing"
)

func TestReverse(t *testing.T) {
	tmpPort := availablePort()
	//setup server, client, fileserver
	teardown := simpleSetup(t,
		&chserver.Config{
			Auth:    "admin:admin",
			Reverse: true,
			/*	TLS: chserver.TLSConfig{
				Domains: []string{"localhost", "127.0.0.1"},
			},*/
		},
		&chclient.Config{
			Remotes: []string{tmpPort + ":127.0.0.1->$FILEPORT"},
			Auth:    "admin:admin",
			//Remotes: []string{"$FILEPORT:127.0.0.1->" + tmpPort},
		})
	defer teardown()
	//test remote (this goes through the server and out the client)
	result, err := post("http://localhost:"+tmpPort, "foo")
	if err != nil {
		t.Fatal(err)
	}
	if result != "foo!" {
		t.Fatalf("expected exclamation mark added")
	}
}

func TestReverseMTLS_SSL(t *testing.T) {
	tlsConfig, err := newTestTLSConfig()
	if err != nil {
		t.Fatal(err)
	}
	defer tlsConfig.Close()
	tmpPort := availablePort()
	//setup server, client, fileserver
	teardown := simpleSetup(t,
		&chserver.Config{
			Auth: "admin:admin",
			TLS:  *tlsConfig.serverTLS,
		},
		&chclient.Config{
			Remotes: []string{tmpPort + "->$FILEPORT"},
			Auth:    "admin:admin",
			TLS:     *tlsConfig.clientTLS,
		})
	defer teardown()
	//test remote (this goes through the server and out the client)
	result, err := postWithTls("https://localhost:"+tmpPort, "foo", tlsConfig)
	if err != nil {
		t.Fatal(err)
	}
	if result != "foo!" {
		t.Fatalf("expected exclamation mark added")
	}
}
