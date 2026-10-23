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
	want := Config{Env: "dev", Port: 8080, SiteOrigin: "http://localhost:3000", APIOrigin: "http://localhost:8080",
		DatabaseURL: "postgres://printing:printing@localhost:5432/printing?sslmode=disable", HashKey: devHashKey}
	if cfg != want {
		t.Errorf("got %+v, want %+v", cfg, want)
	}
}

func TestLoadReadsEveryVariable(t *testing.T) {
	cfg, err := Load(env(map[string]string{
		"ENV": "prod", "PORT": "9000", "SITE_ORIGIN": "https://printingprofiles.com",
		"API_ORIGIN": "https://API.printingprofiles.com/", "DATABASE_URL": "postgres://u:p@db.example:5432/pp?sslmode=require",
		"GITHUB_CLIENT_ID": "gh-id", "GITHUB_CLIENT_SECRET": "gh-secret",
		"DISCORD_CLIENT_ID": "dc-id", "DISCORD_CLIENT_SECRET": "dc-secret",
		"HASH_KEY": "0123456789abcdef0123456789abcdef", "PROXY_HOPS": "2",
	}))

	if err != nil {
		t.Fatal(err)
	}
	want := Config{Env: "prod", Port: 9000, SiteOrigin: "https://printingprofiles.com", APIOrigin: "https://api.printingprofiles.com",
		DatabaseURL: "postgres://u:p@db.example:5432/pp?sslmode=require",
		GitHub:      OAuthClient{ID: "gh-id", Secret: "gh-secret"}, Discord: OAuthClient{ID: "dc-id", Secret: "dc-secret"},
		HashKey: "0123456789abcdef0123456789abcdef", ProxyHops: 2}
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

func TestLoadNeedsBothOAuthValuesAndNeverPrintsTheSecret(t *testing.T) {
	_, err := Load(env(map[string]string{"GITHUB_CLIENT_SECRET": "very-secret-value"}))

	if err == nil || !strings.Contains(err.Error(), "GITHUB_CLIENT_ID") {
		t.Fatalf("err = %v, want a message about GITHUB_CLIENT_ID", err)
	}
	if strings.Contains(err.Error(), "very-secret-value") {
		t.Error("the error must not contain the secret")
	}
}

func TestLoadNeedsAnHTTPSAPIOriginInProd(t *testing.T) {
	base := map[string]string{"ENV": "prod", "SITE_ORIGIN": "https://printingprofiles.com", "DATABASE_URL": "postgres://u:p@h/d",
		"HASH_KEY": "0123456789abcdef0123456789abcdef"}
	if _, err := Load(env(base)); err == nil || !strings.Contains(err.Error(), "API_ORIGIN must be set") {
		t.Errorf("missing API_ORIGIN: err = %v", err)
	}
	base["API_ORIGIN"] = "http://api.printingprofiles.com"
	if _, err := Load(env(base)); err == nil || !strings.Contains(err.Error(), "API_ORIGIN") {
		t.Errorf("http API_ORIGIN in prod: err = %v", err)
	}
}

func TestLoadNeedsALongHashKeyInProdAndOneProxyHopByDefault(t *testing.T) {
	base := map[string]string{"ENV": "prod", "SITE_ORIGIN": "https://printingprofiles.com", "API_ORIGIN": "https://api.printingprofiles.com",
		"DATABASE_URL": "postgres://u:p@h/d"}
	if _, err := Load(env(base)); err == nil || !strings.Contains(err.Error(), "HASH_KEY must be set") {
		t.Errorf("missing HASH_KEY: err = %v", err)
	}
	base["HASH_KEY"] = "short-secret"
	if _, err := Load(env(base)); err == nil || strings.Contains(err.Error(), "short-secret") {
		t.Errorf("short HASH_KEY: err = %v, want an error without the value", err)
	}
	base["HASH_KEY"] = "0123456789abcdef0123456789abcdef"
	cfg, err := Load(env(base))
	if err != nil || cfg.ProxyHops != 1 {
		t.Errorf("prod defaults: ProxyHops %d, err %v, want 1", cfg.ProxyHops, err)
	}
}

func TestLoadReadsSMTPAndNeedsItsLoginWhenAHostIsSet(t *testing.T) {
	cfg, err := Load(env(map[string]string{"SMTP_HOST": "smtp.resend.com", "SMTP_PORT": "465", "SMTP_USERNAME": "resend",
		"SMTP_PASSWORD": "re_secret_value", "MAIL_FROM": "Printing Profiles <signin@printingprofiles.com>"}))
	if err != nil || cfg.SMTP != (SMTP{Host: "smtp.resend.com", Port: 465, Username: "resend", Password: "re_secret_value", From: "Printing Profiles <signin@printingprofiles.com>"}) {
		t.Errorf("SMTP = %+v, err %v", cfg.SMTP, err)
	}
	_, err = Load(env(map[string]string{"SMTP_HOST": "smtp.resend.com", "SMTP_PASSWORD": "re_secret_value"}))
	if err == nil || !strings.Contains(err.Error(), "MAIL_FROM") || strings.Contains(err.Error(), "re_secret_value") {
		t.Errorf("missing sender and login: err = %v", err)
	}
	if cfg, _ := Load(env(nil)); cfg.SMTP != (SMTP{}) {
		t.Errorf("no SMTP_HOST: SMTP = %+v, want empty", cfg.SMTP)
	}
}

func TestLoadReadsR2AsOneSetAndNeverPrintsTheSecret(t *testing.T) {
	all := map[string]string{
		"R2_ENDPOINT": "https://account.r2.cloudflarestorage.com", "R2_ACCESS_KEY_ID": "key-id", "R2_SECRET_ACCESS_KEY": "r2-secret-value",
		"R2_UPLOADS_BUCKET": "printing-uploads", "R2_FILES_BUCKET": "printing-files", "FILES_ORIGIN": "https://files.printingprofiles.com",
	}
	cfg, err := Load(env(all))
	want := R2{Endpoint: "https://account.r2.cloudflarestorage.com", AccessKeyID: "key-id", SecretAccessKey: "r2-secret-value",
		UploadsBucket: "printing-uploads", FilesBucket: "printing-files", FilesOrigin: "https://files.printingprofiles.com"}
	if err != nil || cfg.R2 != want {
		t.Errorf("R2 = %+v, err %v", cfg.R2, err)
	}
	delete(all, "R2_FILES_BUCKET")
	if _, err := Load(env(all)); err == nil || !strings.Contains(err.Error(), "R2_") || strings.Contains(err.Error(), "r2-secret-value") {
		t.Errorf("a missing R2 value: err = %v", err)
	}
	if cfg, _ := Load(env(nil)); cfg.R2 != (R2{}) {
		t.Errorf("no R2 variables: R2 = %+v, want empty", cfg.R2)
	}
}
