package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	chadmin "github.com/NextChapterSoftware/chissl/admin"
	chclient "github.com/NextChapterSoftware/chissl/client"
	chserver "github.com/NextChapterSoftware/chissl/server"
	chshare "github.com/NextChapterSoftware/chissl/share"
	"github.com/NextChapterSoftware/chissl/share/auth"
	"github.com/NextChapterSoftware/chissl/share/ccrypto"
	"github.com/NextChapterSoftware/chissl/share/cos"
	"github.com/NextChapterSoftware/chissl/share/database"
	"github.com/NextChapterSoftware/chissl/share/settings"
)

var help = `
  Usage: chissl [command] [--help]

  Version: ` + chshare.BuildVersion + ` (` + runtime.Version() + `)

  Commands:
    server - runs chissl in server mode
    client - runs chissl in client mode
    admin  - runs local admin client for user management

  Read more:
    https://github.com/NextChapterSoftware/chissl

`

func main() {

	version := flag.Bool("version", false, "")
	v := flag.Bool("v", false, "")
	flag.Bool("help", false, "")
	flag.Bool("h", false, "")
	flag.Usage = func() {}
	flag.Parse()

	if *version || *v {
		fmt.Println(chshare.BuildVersion)
		os.Exit(0)
	}

	args := flag.Args()

	subcmd := ""
	if len(args) > 0 {
		subcmd = args[0]
		args = args[1:]
	}

	switch subcmd {
	case "server":
		server(args)
	case "client":
		client(args)
	case "admin":
		admin(args)
	default:
		fmt.Print(help)
		os.Exit(0)
	}
}

var commonHelp = `
    --pid Generate pid file in current working directory

    -v, Enable verbose logging

    --help, This help text

  Signals:
    The chissl process is listening for:
      a SIGUSR2 to print process stats, and
      a SIGHUP to short-circuit the client reconnect timer

  Version:
    ` + chshare.BuildVersion + ` (` + runtime.Version() + `)

  Read more:
    https://github.com/NextChapterSoftware/chissl

`

func generatePidFile() {
	pid := []byte(strconv.Itoa(os.Getpid()))
	if err := os.WriteFile("chisel.pid", pid, 0644); err != nil {
		log.Fatal(err)
	}
}

var serverHelp = `
  Usage: chissl server [options]

  Options:

    --host, Defines the HTTP listening host – the network interface
    (defaults the environment variable HOST and falls back to 0.0.0.0).

    --port, -p, Defines the HTTP listening port (defaults to the environment
    variable PORT and fallsback to port 8080).

    --key, (deprecated use --keygen and --keyfile instead)
    An optional string to seed the generation of a ECDSA public
    and private key pair. All communications will be secured using this
    key pair. Share the subsequent fingerprint with clients to enable detection
    of man-in-the-middle attacks (defaults to the CHISEL_KEY environment
    variable, otherwise a new key is generate each run).

    --keygen, A path to write a newly generated PEM-encoded SSH private key file.
    If users depend on your --key fingerprint, you may also include your --key to
    output your existing key. Use - (dash) to output the generated key to stdout.

    --keyfile, An optional path to a PEM-encoded SSH private key. When
    this flag is set, the --key option is ignored, and the provided private key
    is used to secure all communications. (defaults to the CHISEL_KEY_FILE
    environment variable). Since ECDSA keys are short, you may also set keyfile
    to an inline base64 private key (e.g. chissl server --keygen - | base64).

    --authfile, An optional path to a users.json file. This file should
    be an object with users defined like:
      {
        "<user:pass>": ["<addr-regex>","<addr-regex>"]
      }
    when <user> connects, their <pass> will be verified and then
    each of the remote addresses will be compared against the list
    of address regular expressions for a match. Addresses will always
    come in the form:
        "local-port:local-host->remote-port:remote-host"
    This file will be automatically reloaded on change.

    --auth, An optional string representing a single user with full
    access, in the form of <user:pass>. It is equivalent to creating an
    authfile with {"<user:pass>": [""]}. If unset, it will use the
    environment variable AUTH.

    --keepalive, An optional keepalive interval. Since the underlying
    transport is HTTP, in many instances we'll be traversing through
    proxies, often these proxies will close idle connections. You must
    specify a time with a unit, for example '5s' or '2m'. Defaults
    to '25s' (set to 0s to disable).

    --tls-key, Enables TLS and provides optional path to a PEM-encoded
    TLS private key. When this flag is set, you must also set --tls-cert,
    and you cannot set --tls-domain.

    --tls-cert, Enables TLS and provides optional path to a PEM-encoded
    TLS certificate. When this flag is set, you must also set --tls-key,
    and you cannot set --tls-domain.

    --tls-domain, Enables TLS and automatically acquires a TLS key and
    certificate using LetsEncrypt. Setting --tls-domain requires port 443.
    You may specify multiple --tls-domain flags to serve multiple domains.
    The resulting files are cached in the "$HOME/.cache/chisel" directory.
    You can modify this path by setting the CHISEL_LE_CACHE variable,
    or disable caching by setting this variable to "-". You can optionally
    provide a certificate notification email by setting CHISEL_LE_EMAIL.

    --tls-ca, a path to a PEM encoded CA certificate bundle or a directory
    holding multiple PEM encode CA certificate bundle files, which is used to
    validate client connections. The provided CA certificates will be used
    instead of the system roots. This is commonly used to implement mutual-TLS.

    --db-type, Database type (sqlite or postgres). Defaults to sqlite.

    --db-file, SQLite database file path. Defaults to ./chissl.db.

    --db-host, Database host for PostgreSQL. Defaults to localhost.

    --db-port, Database port for PostgreSQL. Defaults to 5432.

    --db-name, Database name for PostgreSQL. Defaults to chissl.

    --db-user, Database username for PostgreSQL.

    --db-pass, Database password for PostgreSQL.

    --db-ssl, Database SSL mode for PostgreSQL. Defaults to disable.

    --auth0-enabled, Enable Auth0 SSO integration.

    --auth0-domain, Auth0 domain (e.g., your-tenant.auth0.com).

    --auth0-client-id, Auth0 application client ID.

    --auth0-client-secret, Auth0 application client secret.

    --auth0-audience, Auth0 API audience identifier.

    --dashboard, Enable the web dashboard interface.

    --dashboard-path, URL path for the dashboard. Defaults to /dashboard.
` + commonHelp

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
	flags.StringVar(&config.TLS.CA, "tls-ca", "", "")

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

type multiFlag struct {
	values *[]string
}

func (flag multiFlag) String() string {
	return strings.Join(*flag.values, ", ")
}

func (flag multiFlag) Set(arg string) error {
	*flag.values = append(*flag.values, arg)
	return nil
}

type headerFlags struct {
	http.Header
}

func (flag *headerFlags) String() string {
	out := ""
	for k, v := range flag.Header {
		out += fmt.Sprintf("%s: %s\n", k, v)
	}
	return out
}

func (flag *headerFlags) Set(arg string) error {
	index := strings.Index(arg, ":")
	if index < 0 {
		return fmt.Errorf(`Invalid header (%s). Should be in the format "HeaderName: HeaderContent"`, arg)
	}
	if flag.Header == nil {
		flag.Header = http.Header{}
	}
	key := arg[0:index]
	value := arg[index+1:]
	flag.Header.Set(key, strings.TrimSpace(value))
	return nil
}

var clientHelp = `
  Usage: chissl client [options] <server> <remote> [remote] [remote] ...

  <server> is the URL to the chissl server.

  <remote>s are remote connections tunneled through the server, each of
  which come in the form:
   	local-port:local-host->remote-port:remote-host

    ■ local-port (port on server) is required*.
    ■ local-host (interface on server) defaults to 0.0.0.0 (all interfaces).
    ■ remote-port is required*.
    ■ remote-host defaults to 127.0.0.1

    example remotes
      8080->80
      8080:0.0.0.0->80
      8089->80:neverssl.com

  Options:
    --profile, path to profile configuration yaml file. Defaults to
    $HOME/chissl/profile.yaml. Profile yaml file allows users to
     set all client arguments and configurations using a static file.

    YAML Options:
    ---
    fingerprint: "sample_fingerprint"
    auth: "user:password"
    keepalive: 30s
    max-retry-count: 10
    max-retry-interval: 2m
    server: "example.com"
    proxy: "http://admin:password@my-server.com:8081"
    remotes:
      - 8089->80:neverssl.com
      - 8080->80
    headers:
      Foo: ["Bar"]
    tls:
      tls-skip-verify: true
      tls-ca: "/path/to/ca"
      tls-cert: "/path/to/cert"
      tls-key: "/path/to/key"
      hostname: "example.com"
    verbose: true

    --fingerprint, A *strongly recommended* fingerprint string
    to perform host-key validation against the server's public key.
    Fingerprint mismatches will close the connection.
    Fingerprints are generated by hashing the ECDSA public key using
    SHA256 and encoding the result in base64.
    Fingerprints must be 44 characters containing a trailing equals (=).

    --auth, An optional username and password (client authentication)
    in the form: "<user>:<pass>". These credentials are compared to
    the credentials inside the server's --authfile. defaults to the
    AUTH environment variable.

    --keepalive, An optional keepalive interval. Since the underlying
    transport is HTTP, in many instances we'll be traversing through
    proxies, often these proxies will close idle connections. You must
    specify a time with a unit, for example '5s' or '2m'. Defaults
    to '25s' (set to 0s to disable).

    --max-retry-count, Maximum number of times to retry before exiting.
    Defaults to unlimited.

    --max-retry-interval, Maximum wait time before retrying after a
    disconnection. Defaults to 5 minutes.

    --proxy, An optional HTTP CONNECT or SOCKS5 proxy which will be
    used to reach the chissl server. Authentication can be specified
    inside the URL.
    For example, http://admin:password@my-server.com:8081
            or: socks://admin:password@my-server.com:1080

    --hostname, Optionally set the 'Host' header (defaults to the host
    found in the server url).

    --sni, Override the ServerName when using TLS (defaults to the
    hostname).

    --tls-ca, An optional root certificate bundle used to verify the
    chissl server. Only valid when connecting to the server with
    "https" or "wss". By default, the operating system CAs will be used.

    --tls-skip-verify, Skip server TLS certificate verification of
    chain and host name (if TLS is used for transport connections to
    server). If set, client accepts any TLS certificate presented by
    the server and any host name in that certificate. This only affects
    transport https (wss) connection. Chisel server's public key
    may be still verified (see --fingerprint) after inner connection
    is established.

    --tls-key, a path to a PEM encoded private key used for client
    authentication (mutual-TLS).

    --tls-cert, a path to a PEM encoded certificate matching the provided
    private key. The certificate must have client authentication
    enabled (mutual-TLS).
` + commonHelp

func client(args []string) {
	flags := flag.NewFlagSet("client", flag.ContinueOnError)

	config := &chclient.Config{Headers: http.Header{}}
	profilePath := flags.String("profile", "", "")
	flags.StringVar(&config.Fingerprint, "fingerprint", "", "")
	flags.StringVar(&config.Auth, "auth", "", "")
	flags.DurationVar(&config.KeepAlive, "keepalive", 25*time.Second, "")
	flags.IntVar(&config.MaxRetryCount, "max-retry-count", -1, "")
	flags.DurationVar(&config.MaxRetryInterval, "max-retry-interval", 0, "")
	flags.StringVar(&config.Proxy, "proxy", "", "")
	flags.StringVar(&config.TLS.CA, "tls-ca", "", "")
	flags.BoolVar(&config.TLS.SkipVerify, "tls-skip-verify", false, "")
	flags.StringVar(&config.TLS.Cert, "tls-cert", "", "")
	flags.StringVar(&config.TLS.Key, "tls-key", "", "")
	//flags.Var(&headerFlags{config.Headers}, "header", "")
	hostname := flags.String("hostname", "", "")
	sni := flags.String("sni", "", "")
	pid := flags.Bool("pid", false, "")
	verbose := flags.Bool("v", false, "")
	flags.Usage = func() {
		fmt.Print(clientHelp)
		os.Exit(0)
	}
	flags.Parse(args)
	config, err := chclient.NewClientConfig(*profilePath, config)
	if err != nil {
		log.Fatal(err)
	}

	if len(args) < 3 && *profilePath == "" && config.Server == "" {
		log.Fatalln("No command line arguments or profile yaml provided")
	}

	//pull out options, put back remaining args
	args = flags.Args()
	if len(args) > 0 && *profilePath == "" {
		config.Server = args[0]

	}

	if len(args) > 1 && *profilePath == "" {
		config.Remotes = args[1:]
	}

	//default auth
	if config.Auth == "" {
		config.Auth = os.Getenv("AUTH")
	}
	//move hostname onto headers
	if *hostname != "" {
		config.Headers.Set("Host", *hostname)
		config.TLS.ServerName = *hostname
	}

	if *sni != "" {
		config.TLS.ServerName = *sni
	}

	//ready
	c, err := chclient.NewClient(config)
	if err != nil {
		log.Fatal(err)
	}
	c.Debug = *verbose || config.Verbose
	if *pid {
		generatePidFile()
	}
	go cos.GoStats()
	ctx := cos.InterruptContext()
	if err := c.Start(ctx); err != nil {
		log.Fatal(err)
	}
	if err := c.Wait(); err != nil {
		log.Fatal(err)
	}
}

var adminHelp = `
  Usage: chissl admin [subcommand] [options]

  Subcommands:
    adduser - Adds a new user
    deluser - Deletes an existing user
	getuser - Gets info about an existing user
    listusers - Lists all users

  Read more:
    https://github.com/NextChapterSoftware/chissl

`

func admin(args []string) {
	if len(args) == 0 {
		fmt.Print(adminHelp)
		os.Exit(0)
	}

	flags := flag.NewFlagSet("admin", flag.ContinueOnError)
	profilePath := flags.String("profile", "", "Path to profile yaml file")

	flags.Usage = func() {
		fmt.Print(adminHelp)
		os.Exit(1)
	}

	// Parse the flags for the main command
	err := flags.Parse(args)
	if err != nil {
		log.Fatal("Error parsing main command flags:", err)

	}

	config := &chadmin.Config{}
	config, err = chadmin.NewClientConfig(*profilePath, config)
	if err != nil {
		log.Fatal(err)
	}

	a, err := chadmin.NewAdminClient(config)
	if err != nil {
		log.Fatal(err)
	}

	if len(args) < 1 && config.Server == "" {
		log.Fatalln("No command line arguments or profile yaml provided")
	}

	// Pull out used args
	args = flags.Args()

	// Determine the subcommand
	subcommand := args[0]
	subcommandArgs := args[1:]

	switch subcommand {
	case "adduser":
		a.AddUser(subcommandArgs)
	case "deluser":
		a.DelUser(subcommandArgs)
	case "getuser":
		a.GerUser(subcommandArgs)
	case "listusers":
		a.ListUsers(subcommandArgs)
	default:
		fmt.Print(adminHelp)
		os.Exit(0)
	}
}
