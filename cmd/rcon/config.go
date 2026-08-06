package main

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
)

// DefaultPort is the conventional Source RCON TCP port.
const DefaultPort = 25575

// ServerConfig is one named server in the config file.
type ServerConfig struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Password     string `json:"password"`
	SinglePacket bool   `json:"singlePacket"`
	Drain        bool   `json:"drain"`
}

// Config is the on-disk JSON config: named servers plus a default selection.
type Config struct {
	Default string                  `json:"default"`
	Servers map[string]ServerConfig `json:"servers"`
}

// Flags holds the raw command-line inputs relevant to connection resolution.
type Flags struct {
	Host         string
	Port         int
	Password     string
	Server       string // selects a named server from the config
	SinglePacket bool
	Drain        bool
}

// Env holds the relevant environment variables.
type Env struct {
	Host         string
	Port         int
	Password     string
	SinglePacket bool
	Drain        bool
}

// Resolved is the final connection target after applying precedence.
type Resolved struct {
	Address      string
	Password     string
	SinglePacket bool
	Drain        bool
}

// LoadConfig reads a JSON config from path. A missing file is only an error when
// path was explicitly provided (mustExist).
func LoadConfig(path string, mustExist bool) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && !mustExist {
			return Config{}, nil
		}
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, nil
}

// Resolve applies precedence flags > env > file > defaults and returns the
// connection target.
func Resolve(cfg Config, flags Flags, env Env) (Resolved, error) {
	// Start from the named server (or the config default). Precedence for each
	// field is flags, then env, then the config file, then a default.
	base := cfg.Servers[cmp.Or(flags.Server, cfg.Default)]

	host := cmp.Or(flags.Host, env.Host, base.Host)
	password := cmp.Or(flags.Password, env.Password, base.Password)
	port := cmp.Or(flags.Port, env.Port, base.Port, DefaultPort)

	if host == "" {
		return Resolved{}, errors.New("no host specified (use --host, RCON_HOST, or a config server)")
	}
	// SinglePacket and Drain are boolean opt-ins, so each is enabled if any
	// source turns it on (flag, env, or the chosen server) rather than following
	// the usual first-non-zero precedence, which could never re-disable them.
	return Resolved{
		Address:      net.JoinHostPort(host, strconv.Itoa(port)),
		Password:     password,
		SinglePacket: flags.SinglePacket || env.SinglePacket || base.SinglePacket,
		Drain:        flags.Drain || env.Drain || base.Drain,
	}, nil
}
