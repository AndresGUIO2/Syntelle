package accounts

import (
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type Config struct {
	Secret             string
	Origin             string
	ResendAPIKey       string
	ResendFrom         string
	GoogleClientID     string
	GoogleClientSecret string
	GitHubClientID     string
	GitHubClientSecret string
	PackageHash        string
	HTTPClient         *http.Client
	Mailer             Mailer
	Now                func() time.Time
}

func ConfigFromEnv() Config {
	origin := os.Getenv("SYNTELLE_APP_ORIGIN")
	if origin == "" {
		origin = "http://127.0.0.1:5173"
	}
	return Config{
		Secret: os.Getenv("SYNTELLE_AUTH_SECRET"), Origin: strings.TrimRight(origin, "/"),
		ResendAPIKey: os.Getenv("RESEND_API_KEY"), ResendFrom: os.Getenv("RESEND_FROM"),
		GoogleClientID: os.Getenv("GOOGLE_CLIENT_ID"), GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GitHubClientID: os.Getenv("GITHUB_CLIENT_ID"), GitHubClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
		PackageHash: os.Getenv("SYNTELLE_PACKAGE_HASH"),
	}
}

func (s *Service) secureCookie() bool {
	u, err := url.Parse(s.cfg.Origin)
	return err != nil || u.Scheme == "https" || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost" && u.Hostname() != "::1")
}
