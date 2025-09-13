package e2e_test

import (
	"testing"

	chclient "github.com/NextChapterSoftware/chissl/client"
	chserver "github.com/NextChapterSoftware/chissl/server"
)

//TODO tests for:
// - failed auth
// - dynamic auth (server add/remove user)
// - watch auth file

func TestAuth(t *testing.T) {
	tmpPort1 := availablePort()
	tmpPort2 := availablePort()
	//setup server, client, fileserver
	teardown := simpleSetup(t,
		&chserver.Config{
			KeySeed: "foobar",
			Auth:    "foo:bar",
		},
		&chclient.Config{
			Remotes: []string{
				tmpPort1 + ":0.0.0.0->$FILEPORT:127.0.0.1",
				tmpPort2 + ":0.0.0.0->$FILEPORT:localhost",
			},
			Auth: "foo:bar",
		})
	defer teardown()
	//test first remote
	result, err := post("http://localhost:"+tmpPort1, "foo")
	if err != nil {
		t.Fatal(err)
	}
	if result != "foo!" {
		t.Fatalf("expected exclamation mark added")
	}
	//test second remote
	result, err = post("http://localhost:"+tmpPort2, "bar")
	if err != nil {
		t.Fatal(err)
	}
	if result != "bar!" {
		t.Fatalf("expected exclamation mark added again")
	}
}
