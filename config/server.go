package config

import "strings"

// SessionMode selects which authority owns the mobile runtime. It is kept in
// config rather than the renderer so desktop and Android can share the same
// lifecycle rules.
type SessionMode string

const (
	SessionModeOnline  SessionMode = "online"
	SessionModeOffline SessionMode = "offline"
)

// ServerConfig describes one directly configured game server. The catalog
// abstraction can later supply these values without changing the session or
// packet layers.
type ServerConfig struct {
	Name       string
	Host       string
	AuthPort   int
	CharPort   int
	ZonePort   int
	Username   string
	Password   string
	ClientDate int
	Profile    int
}

func (s ServerConfig) Normalized() ServerConfig {
	s.Name = strings.TrimSpace(s.Name)
	s.Host = strings.TrimSpace(s.Host)
	if s.Name == "" {
		s.Name = "Local server"
	}
	if s.AuthPort == 0 {
		s.AuthPort = 6900
	}
	if s.CharPort == 0 {
		s.CharPort = 6121
	}
	if s.ZonePort == 0 {
		s.ZonePort = 5121
	}
	if s.ClientDate == 0 {
		s.ClientDate = 20080910
	}
	if s.Profile == 0 {
		s.Profile = 23
	}
	return s
}

// ServerCatalogProvider is the future registry seam. Providers are
// read-only: selecting a server must not grant them gameplay authority.
type ServerCatalogProvider interface {
	Servers() []ServerConfig
}
