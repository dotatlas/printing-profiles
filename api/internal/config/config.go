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
}

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
