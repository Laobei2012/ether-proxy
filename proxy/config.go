package proxy

import "ether-proxy/agent"

type Config struct {
	Proxy                     Proxy      `json:"proxy"`
	Frontend                  Frontend   `json:"frontend"`
	Upstream                  []Upstream `json:"upstream"`
	UpstreamCheckInterval     string     `json:"upstreamCheckInterval"`
	UpstreamProto             string     `json:"upstreamProto"`
	UpstreamMaxNotifyInterval string     `json:"upstreamMaxNotifyInterval"`

	Threads int `json:"threads"`

	// Agent is only used in agent mode (-mode 0); the proxy never connects to MQTT.
	Agent agent.Config `json:"agent"`

	NewrelicName    string `json:"newrelicName"`
	NewrelicKey     string `json:"newrelicKey"`
	NewrelicVerbose bool   `json:"newrelicVerbose"`
	NewrelicEnabled bool   `json:"newrelicEnabled"`
}

type Proxy struct {
	Listen               string `json:"listen"`
	ClientTimeout        string `json:"clientTimeout"`
	BlockRefreshInterval string `json:"blockRefreshInterval"`
	HashrateWindow       string `json:"hashrateWindow"`
	SubmitHashrate       bool   `json:"submitHashrate"`
	LuckWindow           string `json:"luckWindow"`
	LargeLuckWindow      string `json:"largeLuckWindow"`

	MaxFails    int64 `json:"maxFails"`
	HealthCheck bool  `json:"healthCheck"`

	Stratum Stratum `json:"stratum"`
	TLS     TLS     `json:"tls"`
}

type TLS struct {
	Enabled bool   `json:"enabled"`
	Listen  string `json:"listen"`
	PemFile string `json:"pemFile"`
	KeyFile string `json:"keyFile"`
}

type Stratum struct {
	Enabled bool   `json:"enabled"`
	Listen  string `json:"listen"`
	Timeout string `json:"timeout"`
	MaxConn int    `json:"maxConn"`
}

type Frontend struct {
	Listen   string `json:"listen"`
	Login    string `json:"login"`
	Password string `json:"password"`
}

type Upstream struct {
	Name    string `json:"name"`
	Url     string `json:"url"`
	Scheme  string
	Host    string
	Port    string
	User    string
	Timeout string `json:"timeout"`
	Pool    bool   `json:"pool"`
}
