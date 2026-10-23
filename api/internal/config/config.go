// Package config reads the API settings from environment variables.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Config holds the API settings. main reads them one time, before the server starts.
type Config struct {
	Env         string // "dev" or "prod"
	Port        int    // the TCP port that the API listens on
	SiteOrigin  string // the only origin that CORS allows, for example https://printingprofiles.com
	APIOrigin   string // the API's own origin, for the OAuth callback URLs
	DatabaseURL string // the PostgreSQL connection URL
	GitHub      OAuthClient
	Discord     OAuthClient
	HashKey     string // the HMAC key for rate-limit keys, so stored IPs and addresses cannot be read back
	ProxyHops   int    // how many proxies in front add to X-Forwarded-For; 0 means use the socket address
	SMTP        SMTP   // the relay for sign-in emails; with no Host, prod has no email sign-in and dev logs the emails
}

// SMTP is the relay that sends the sign-in emails.
type SMTP struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string // the sender, for example "Printing Profiles <signin@printingprofiles.com>"
}

// devHashKey is the HMAC key in dev. It is not a secret, so prod must set its own HASH_KEY.
const devHashKey = "dev-only-hash-key-that-is-not-a-secret"

// OAuthClient is the client ID and secret of one OAuth app. A provider is on only when both are set.
type OAuthClient struct {
	ID     string
	Secret string
}

// Load reads the settings with getenv and checks them. main calls it with os.Getenv, and it returns all errors together.
func Load(getenv func(string) string) (Config, error) {
	// These defaults let the API run in dev with no settings.
	cfg := Config{
		Env:         "dev",
		Port:        8080,
		SiteOrigin:  "http://localhost:3000",
		APIOrigin:   "http://localhost:8080",
		HashKey:     devHashKey,
		DatabaseURL: "postgres://printing:printing@localhost:5432/printing?sslmode=disable", // the docker compose database
	}
	var errs []error

	if v := getenv("ENV"); v != "" {
		cfg.Env = v
	}
	if cfg.Env != "dev" && cfg.Env != "prod" {
		errs = append(errs, fmt.Errorf("ENV must be dev or prod, not %q", cfg.Env))
	}

	if v := getenv("PORT"); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil || port < 1 || port > 65535 {
			errs = append(errs, fmt.Errorf("PORT must be a number from 1 to 65535, not %q", v))
		} else {
			cfg.Port = port
		}
	}

	cfg.SiteOrigin, errs = readOrigin(getenv, "SITE_ORIGIN", cfg.SiteOrigin, cfg.Env == "prod", errs)
	cfg.APIOrigin, errs = readOrigin(getenv, "API_ORIGIN", cfg.APIOrigin, cfg.Env == "prod", errs)
	cfg.GitHub, errs = readOAuthClient(getenv, "GITHUB", errs)
	cfg.Discord, errs = readOAuthClient(getenv, "DISCORD", errs)

	key := getenv("HASH_KEY")
	switch {
	case key == "" && cfg.Env == "prod":
		errs = append(errs, errors.New("HASH_KEY must be set in prod"))
	case key != "" && len(key) < 32:
		errs = append(errs, errors.New("HASH_KEY must be at least 32 characters")) // The value stays out of the error.
	case key != "":
		cfg.HashKey = key
	}

	cfg.SMTP, errs = readSMTP(getenv, errs)

	if cfg.Env == "prod" {
		cfg.ProxyHops = 1 // In prod, the host's ingress is the one proxy in front of the API.
	}
	if v := getenv("PROXY_HOPS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n > 5 {
			errs = append(errs, fmt.Errorf("PROXY_HOPS must be a number from 0 to 5, not %q", v))
		} else {
			cfg.ProxyHops = n
		}
	}
	dbURL := getenv("DATABASE_URL")
	switch {
	case dbURL == "" && cfg.Env == "prod":
		errs = append(errs, errors.New("DATABASE_URL must be set in prod"))
	case dbURL != "":
		if u, err := url.Parse(dbURL); err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
			// Do not put the URL in the error: it can contain a password.
			errs = append(errs, errors.New("DATABASE_URL must be a postgres:// URL"))
		} else {
			cfg.DatabaseURL = dbURL
		}
	}
	return cfg, errors.Join(errs...)
}

// readOrigin reads the origin in the variable name, or keeps def when it is not set. In prod, the variable is required. Load calls it.
func readOrigin(getenv func(string) string, name, def string, prod bool, errs []error) (string, []error) {
	v := getenv(name)
	switch {
	case v == "" && prod:
		return def, append(errs, fmt.Errorf("%s must be set in prod", name))
	case v == "":
		return def, errs
	}
	o, err := parseOrigin(v, prod)
	if err != nil {
		return def, append(errs, fmt.Errorf("%s: %w", name, err))
	}
	return o, errs
}

// readSMTP reads the SMTP_ variables and MAIL_FROM. With SMTP_HOST set, the sender and the login are required. Load calls it.
func readSMTP(getenv func(string) string, errs []error) (SMTP, []error) {
	m := SMTP{Host: getenv("SMTP_HOST"), Port: 587, Username: getenv("SMTP_USERNAME"), Password: getenv("SMTP_PASSWORD"), From: getenv("MAIL_FROM")}
	if m.Host == "" {
		return SMTP{}, errs
	}
	if v := getenv("SMTP_PORT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 65535 {
			errs = append(errs, fmt.Errorf("SMTP_PORT must be a number from 1 to 65535, not %q", v))
		}
		m.Port = n
	}
	if m.From == "" || m.Username == "" || m.Password == "" {
		// Do not put the values in the error: the password is a secret.
		errs = append(errs, errors.New("with SMTP_HOST set, MAIL_FROM, SMTP_USERNAME and SMTP_PASSWORD must be set too"))
	}
	return m, errs
}

// readOAuthClient reads <prefix>_CLIENT_ID and <prefix>_CLIENT_SECRET, which must both be set or both be empty. Load calls it.
func readOAuthClient(getenv func(string) string, prefix string, errs []error) (OAuthClient, []error) {
	c := OAuthClient{ID: getenv(prefix + "_CLIENT_ID"), Secret: getenv(prefix + "_CLIENT_SECRET")}
	if (c.ID == "") != (c.Secret == "") {
		// Do not put the values in the error: one of them is a secret.
		return OAuthClient{}, append(errs, fmt.Errorf("set both %s_CLIENT_ID and %s_CLIENT_SECRET, or neither", prefix, prefix))
	}
	return c, errs
}

// parseOrigin checks that s is an origin and returns it in lower case. readOrigin uses it.
func parseOrigin(s string, httpsOnly bool) (string, error) {
	u, err := url.Parse(s)
	if err != nil {
		return "", err
	}
	if u.Scheme != "https" && (httpsOnly || u.Scheme != "http") {
		return "", fmt.Errorf("%q must start with https:// (http:// is allowed only in dev)", s)
	}
	// An origin has only a scheme and a host. One "/" at the end is allowed, and Load removes it.
	if u.Host == "" || u.User != nil || u.Opaque != "" || (u.Path != "" && u.Path != "/") ||
		u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return "", fmt.Errorf("%q must be only a scheme and a host, like https://printingprofiles.com", s)
	}
	return u.Scheme + "://" + strings.ToLower(u.Host), nil
}
