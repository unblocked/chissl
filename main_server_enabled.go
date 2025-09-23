//go:build server
// +build server

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	chserver "github.com/NextChapterSoftware/chissl/server"
	"github.com/NextChapterSoftware/chissl/share/auth"
	"github.com/NextChapterSoftware/chissl/share/ccrypto"
	"github.com/NextChapterSoftware/chissl/share/cos"
	"github.com/NextChapterSoftware/chissl/share/database"
	"github.com/NextChapterSoftware/chissl/share/settings"
)

// server is only compiled when the 'server' build tag is set.
func server(args []string) {
	flags := flag.NewFlagSet("server", flag.ContinueOnError)

	config := &chserver.Config{
		Database: &database.DatabaseConfig{},
		Auth0:    &auth.Auth0Config{},
	}
	flags.StringVar(&config.KeySeed, "key", "", "")
	flags.StringVar(&config.KeyFile, "keyfile", "", "")
	flags.StringVar(&config.AuthFile, "authfile", "", "")
	flags.StringVar(&config.Auth, "auth", "", "")
	flags.DurationVar(&config.KeepAlive, "keepalive", 25*time.Second, "")
	flags.StringVar(&config.Proxy, "proxy", "", "")
	flags.StringVar(&config.TLS.Key, "tls-key", "", "")
	flags.StringVar(&config.TLS.Cert, "tls-cert", "", "")
	flags.Var(multiFlag{&config.TLS.Domains}, "tls-domain", "")
	flags.StringVar(&config.TLS.CA, "tls-ca", "", "TLS CA certificate file (PEM)")

	// Database configuration
	flags.StringVar(&config.Database.Type, "db-type", "sqlite", "Database type (sqlite or postgres)")
	flags.StringVar(&config.Database.FilePath, "db-file", "./chissl.db", "SQLite database file path")
	flags.StringVar(&config.Database.Host, "db-host", "localhost", "Database host")
	flags.IntVar(&config.Database.Port, "db-port", 5432, "Database port")
	flags.StringVar(&config.Database.Database, "db-name", "chissl", "Database name")
	flags.StringVar(&config.Database.Username, "db-user", "", "Database username")
	flags.StringVar(&config.Database.Password, "db-pass", "", "Database password")
	flags.StringVar(&config.Database.SSLMode, "db-ssl", "disable", "Database SSL mode")

	// Auth0 configuration
	flags.BoolVar(&config.Auth0.Enabled, "auth0-enabled", false, "Enable Auth0 integration")
	flags.StringVar(&config.Auth0.Domain, "auth0-domain", "", "Auth0 domain")
	flags.StringVar(&config.Auth0.ClientID, "auth0-client-id", "", "Auth0 client ID")
	flags.StringVar(&config.Auth0.ClientSecret, "auth0-client-secret", "", "Auth0 client secret")
	flags.StringVar(&config.Auth0.Audience, "auth0-audience", "", "Auth0 audience")

	// Dashboard configuration
	flags.BoolVar(&config.Dashboard.Enabled, "dashboard", false, "Enable web dashboard")
	flags.StringVar(&config.Dashboard.Path, "dashboard-path", "/dashboard", "Dashboard URL path")

	host := flags.String("host", "", "")
	p := flags.String("p", "", "")
	port := flags.String("port", "", "")
	pid := flags.Bool("pid", false, "")
	verbose := flags.Bool("v", false, "")
	keyGen := flags.String("keygen", "", "")

	flags.Usage = func() {
		fmt.Print(serverHelp)
		os.Exit(0)
	}
	flags.Parse(args)

	if *keyGen != "" {
		if err := ccrypto.GenerateKeyFile(*keyGen, config.KeySeed); err != nil {
			log.Fatal(err)
		}
		return
	}

	if config.KeySeed != "" {
		log.Print("Option `--key` is deprecated and will be removed in a future version of chisel.")
		log.Print("Please use `chissl server --keygen /file/path`, followed by `chissl server --keyfile /file/path` to specify the SSH private key")
	}

	config.Reverse = true
	if *host == "" {
		*host = os.Getenv("HOST")
	}
	if *host == "" {
		*host = "0.0.0.0"
	}
	if *port == "" {
		*port = *p
	}
	if *port == "" {
		*port = os.Getenv("PORT")
	}
	if *port == "" {
		*port = "443"
	}
	if config.KeyFile == "" {
		config.KeyFile = settings.Env("KEY_FILE")
	} else if config.KeySeed == "" {
		config.KeySeed = settings.Env("KEY")
	}
	s, err := chserver.NewServer(config)
	if err != nil {
		log.Fatal(err)
	}
	s.Debug = *verbose
	if *pid {
		generatePidFile()
	}
	go cos.GoStats()
	ctx := cos.InterruptContext()
	if err := s.StartContext(ctx, *host, *port); err != nil {
		log.Fatal(err)
	}
	if err := s.Wait(); err != nil {
		log.Fatal(err)
	}
}
