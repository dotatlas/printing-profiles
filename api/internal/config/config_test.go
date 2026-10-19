package config

import (
	"strings"
	"testing"
)

// env makes a getenv function from a map. The tests use it in place of os.Getenv.
func env(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

func TestLoadUsesDevDefaultsWhenNothingIsSet(t *testing.T) {
	cfg, err := Load(env(nil))

	if err != nil {
		t.Fatal(err)
	}
	want := Config{Env: "dev", Port: 8080, SiteOrigin: "http://localhost:3000",
		DatabaseURL: "postgres://printing:printing@localhost:5432/printing?sslmode=disable"}
	if cfg != want {
		t.Errorf("got %+v, want %+v", cfg, want)
	}
}

func TestLoadReadsEveryVariable(t *testing.T) {
	cfg, err := Load(env(map[string]string{
		"ENV": "prod", "PORT": "9000", "SITE_ORIGIN": "https://printingprofiles.com",
		"DATABASE_URL": "postgres://u:p@db.example:5432/pp?sslmode=require",
	}))

	if err != nil {
		t.Fatal(err)
	}
	want := Config{Env: "prod", Port: 9000, SiteOrigin: "https://printingprofiles.com",
		DatabaseURL: "postgres://u:p@db.example:5432/pp?sslmode=require"}
	if cfg != want {
		t.Errorf("got %+v, want %+v", cfg, want)
	}
}

func TestLoadNormalizesTheSiteOrigin(t *testing.T) {
	cfg, err := Load(env(map[string]string{"SITE_ORIGIN": "HTTPS://PrintingProfiles.com/"}))

	if err != nil {
		t.Fatal(err)
	}
	if cfg.SiteOrigin != "https://printingprofiles.com" {
		t.Errorf("SiteOrigin = %q, want https://printingprofiles.com", cfg.SiteOrigin)
	}
}

func TestLoadRejectsBadValues(t *testing.T) {
	cases := map[string]map[string]string{
		"unknown env":            {"ENV": "staging"},
		"port not a number":      {"PORT": "eighty"},
		"port zero":              {"PORT": "0"},
		"port too big":           {"PORT": "65536"},
		"origin without scheme":  {"SITE_ORIGIN": "printingprofiles.com"},
		"origin with a path":     {"SITE_ORIGIN": "https://printingprofiles.com/app"},
		"origin with a query":    {"SITE_ORIGIN": "https://printingprofiles.com?x=1"},
		"origin with ftp scheme": {"SITE_ORIGIN": "ftp://printingprofiles.com"},
		"prod without origin":    {"ENV": "prod"},
		"prod with http origin":  {"ENV": "prod", "SITE_ORIGIN": "http://printingprofiles.com"},
		"prod without database":  {"ENV": "prod", "SITE_ORIGIN": "https://printingprofiles.com"},
		"database not postgres":  {"DATABASE_URL": "mysql://u:p@localhost/pp"},
		"database not a url":     {"DATABASE_URL": "::not a url"},
	}
	for name, vars := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(env(vars)); err == nil {
				t.Errorf("%v: want an error, got nil", vars)
			}
		})
	}
}

func TestLoadReportsEveryBadValueAtOnce(t *testing.T) {
	_, err := Load(env(map[string]string{"ENV": "staging", "PORT": "eighty"}))

	if err == nil {
		t.Fatal("want an error, got nil")
	}
	for _, name := range []string{"ENV", "PORT"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not name %s", err, name)
		}
	}
}

func TestLoadNeverPutsTheDatabaseURLInErrors(t *testing.T) {
	_, err := Load(env(map[string]string{"DATABASE_URL": "mysql://user:secret-password@localhost/pp"}))

	if err == nil {
		t.Fatal("want an error, got nil")
	}
	if strings.Contains(err.Error(), "secret-password") {
		t.Errorf("error %q shows the password", err)
	}
}
