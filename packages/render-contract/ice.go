package rendercontract

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

type ICEServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}

func ICEServers() ([]ICEServer, error) {
	if os.Getenv("SYNTELLE_DEPLOYMENT_MODE") != "cloud" {
		return nil, nil
	}
	turnURLs := strings.TrimSpace(os.Getenv("SYNTELLE_TURN_URLS"))
	secret := os.Getenv("SYNTELLE_TURN_SECRET")
	servers := make([]ICEServer, 0, 2)
	if turnURLs != "" {
		if len(secret) < 32 {
			return nil, errors.New("TURN URLs require a TURN signing secret")
		}
		urls := strings.Split(turnURLs, ",")
		for _, u := range urls {
			if !(strings.HasPrefix(u, "turn:") || strings.HasPrefix(u, "turns:")) || strings.ContainsAny(u, " \t\r\n@") || strings.TrimPrefix(strings.TrimPrefix(u, "turn:"), "turns:") == "" {
				return nil, errors.New("invalid TURN URL")
			}
		}
		user := strconv.FormatInt(time.Now().Add(15*time.Minute).Unix(), 10) + ":syntelle"
		mac := hmac.New(sha1.New, []byte(secret))
		mac.Write([]byte(user))
		servers = append(servers, ICEServer{URLs: urls, Username: user, Credential: base64.StdEncoding.EncodeToString(mac.Sum(nil))})
	}
	if raw := os.Getenv("SYNTELLE_STUN_URLS"); raw != "" {
		stun := strings.Split(raw, ",")
		for _, u := range stun {
			if !(strings.HasPrefix(u, "stun:") || strings.HasPrefix(u, "stuns:")) || strings.ContainsAny(u, " \t\r\n@") || strings.TrimPrefix(strings.TrimPrefix(u, "stun:"), "stuns:") == "" {
				return nil, errors.New("invalid STUN URL")
			}
		}
		servers = append(servers, ICEServer{URLs: stun})
	}
	return servers, nil
}
