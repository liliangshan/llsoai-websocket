package config

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	App       AppConfig       `yaml:"app"`
	Database  DatabaseConfig  `yaml:"database"`
	WebSocket WebSocketConfig `yaml:"websocket"`
	HTTP      HTTPConfig      `yaml:"http"`
	Auth      AuthConfig      `yaml:"auth"`
}

type AppConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type DatabaseConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	Username        string        `yaml:"username"`
	Password        string        `yaml:"password"`
	Name            string        `yaml:"name"`
	Charset         string        `yaml:"charset"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
}

type WebSocketConfig struct {
	Path             string        `yaml:"path"`
	ReadTimeout      time.Duration `yaml:"read_timeout"`
	WriteTimeout     time.Duration `yaml:"write_timeout"`
	HeartbeatTimeout time.Duration `yaml:"heartbeat_timeout"`
	SendQueueSize    int           `yaml:"send_queue_size"`
}

type HTTPConfig struct {
	RequestTimeout         time.Duration `yaml:"request_timeout"`
	HistoryTimeout         time.Duration `yaml:"history_timeout"`
	SSEPingInterval        time.Duration `yaml:"sse_ping_interval"`
	SSEMaxLifetime         time.Duration `yaml:"sse_max_lifetime"`
	MaxHistoryResponseSize int64         `yaml:"max_history_response_bytes"`
	MaxHistoryMessageSize  int64         `yaml:"max_history_message_bytes"`
}

type AuthConfig struct {
	APIKeys    []APIKeyConfig `yaml:"api_keys"`
	JWT        JWTConfig      `yaml:"jwt"`
	BcryptCost int            `yaml:"bcrypt_cost"`
}

type JWTConfig struct {
	Secret     string        `yaml:"secret"`
	Expiration time.Duration `yaml:"expiration"`
}

type APIKeyConfig struct {
	Key    string `yaml:"key"`
	UserID uint64 `yaml:"user_id"`
}

func Load(path string) (*Config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content = []byte(expandEnv(string(content)))
	var cfg Config
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return nil, err
	}
	applyDefaults(&cfg)
	return &cfg, nil
}

func (c Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.App.Host, c.App.Port)
}

func (c DatabaseConfig) DSN() string {
	charset := c.Charset
	if charset == "" {
		charset = "utf8mb4"
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=true&loc=Local", c.Username, c.Password, c.Host, c.Port, c.Name, charset)
}

func applyDefaults(cfg *Config) {
	if cfg.App.Host == "" {
		cfg.App.Host = "0.0.0.0"
	}
	if cfg.App.Port == 0 {
		cfg.App.Port = 28081
	}
	if cfg.WebSocket.Path == "" {
		cfg.WebSocket.Path = "/ws"
	}
	if cfg.WebSocket.ReadTimeout == 0 {
		cfg.WebSocket.ReadTimeout = 60 * time.Second
	}
	if cfg.WebSocket.WriteTimeout == 0 {
		cfg.WebSocket.WriteTimeout = 10 * time.Second
	}
	if cfg.WebSocket.HeartbeatTimeout == 0 {
		cfg.WebSocket.HeartbeatTimeout = 90 * time.Second
	}
	if cfg.WebSocket.SendQueueSize == 0 {
		cfg.WebSocket.SendQueueSize = 256
	}
	if cfg.HTTP.RequestTimeout == 0 {
		cfg.HTTP.RequestTimeout = 120 * time.Second
	}
	if cfg.HTTP.HistoryTimeout == 0 {
		cfg.HTTP.HistoryTimeout = 30 * time.Second
	}
	if cfg.HTTP.SSEPingInterval == 0 {
		cfg.HTTP.SSEPingInterval = 15 * time.Second
	}
	if cfg.HTTP.SSEMaxLifetime == 0 {
		cfg.HTTP.SSEMaxLifetime = 10 * time.Minute
	}
	if cfg.HTTP.MaxHistoryResponseSize == 0 {
		cfg.HTTP.MaxHistoryResponseSize = 2 * 1024 * 1024
	}
	if cfg.HTTP.MaxHistoryMessageSize == 0 {
		cfg.HTTP.MaxHistoryMessageSize = 32 * 1024
	}
	if cfg.Auth.JWT.Secret == "" {
		cfg.Auth.JWT.Secret = "default-secret-change-me"
	}
	if cfg.Auth.JWT.Expiration == 0 {
		cfg.Auth.JWT.Expiration = 720 * time.Hour
	}
	if cfg.Auth.BcryptCost == 0 {
		cfg.Auth.BcryptCost = 10
	}
}

var envPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(?::([^}]*))?}`)

func expandEnv(input string) string {
	return envPattern.ReplaceAllStringFunc(input, func(match string) string {
		parts := envPattern.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		if value, ok := os.LookupEnv(parts[1]); ok {
			return value
		}
		if len(parts) > 2 {
			return parts[2]
		}
		return ""
	})
}

// databaseConfigRaw 避免 UnmarshalYAML 递归的类型别名
type databaseConfigRaw DatabaseConfig

func (d *DatabaseConfig) UnmarshalYAML(value *yaml.Node) error {
	var m map[string]any
	if err := value.Decode(&m); err != nil {
		return err
	}
	connLifetimeStr, _ := m["conn_max_lifetime"].(string)
	delete(m, "conn_max_lifetime")
	bytes, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	// 使用类型别名避免再次触发 UnmarshalYAML
	var alias databaseConfigRaw
	if err := yaml.Unmarshal(bytes, &alias); err != nil {
		return err
	}
	*d = DatabaseConfig(alias)
	if connLifetimeStr != "" {
		d.ConnMaxLifetime, err = time.ParseDuration(connLifetimeStr)
		if err != nil {
			return fmt.Errorf("invalid conn_max_lifetime: %w", err)
		}
	}
	return nil
}

// websocketConfigRaw 避免 UnmarshalYAML 递归的类型别名
type websocketConfigRaw WebSocketConfig

func (w *WebSocketConfig) UnmarshalYAML(value *yaml.Node) error {
	var m map[string]any
	if err := value.Decode(&m); err != nil {
		return err
	}
	readStr, _ := m["read_timeout"].(string)
	writeStr, _ := m["write_timeout"].(string)
	heartStr, _ := m["heartbeat_timeout"].(string)
	delete(m, "read_timeout")
	delete(m, "write_timeout")
	delete(m, "heartbeat_timeout")
	bytes, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	var alias websocketConfigRaw
	if err := yaml.Unmarshal(bytes, &alias); err != nil {
		return err
	}
	*w = WebSocketConfig(alias)
	if readStr != "" {
		w.ReadTimeout, err = time.ParseDuration(readStr)
		if err != nil {
			return fmt.Errorf("invalid read_timeout: %w", err)
		}
	}
	if writeStr != "" {
		w.WriteTimeout, err = time.ParseDuration(writeStr)
		if err != nil {
			return fmt.Errorf("invalid write_timeout: %w", err)
		}
	}
	if heartStr != "" {
		w.HeartbeatTimeout, err = time.ParseDuration(heartStr)
		if err != nil {
			return fmt.Errorf("invalid heartbeat_timeout: %w", err)
		}
	}
	return nil
}

// httpConfigRaw 避免 UnmarshalYAML 递归的类型别名
type httpConfigRaw HTTPConfig

func (h *HTTPConfig) UnmarshalYAML(value *yaml.Node) error {
	var m map[string]any
	if err := value.Decode(&m); err != nil {
		return err
	}
	reqStr, _ := m["request_timeout"].(string)
	histStr, _ := m["history_timeout"].(string)
	pingStr, _ := m["sse_ping_interval"].(string)
	maxLifeStr, _ := m["sse_max_lifetime"].(string)
	delete(m, "request_timeout")
	delete(m, "history_timeout")
	delete(m, "sse_ping_interval")
	delete(m, "sse_max_lifetime")
	bytes, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	var alias httpConfigRaw
	if err := yaml.Unmarshal(bytes, &alias); err != nil {
		return err
	}
	*h = HTTPConfig(alias)
	pairs := map[string]*time.Duration{
		"request_timeout":   &h.RequestTimeout,
		"history_timeout":   &h.HistoryTimeout,
		"sse_ping_interval": &h.SSEPingInterval,
		"sse_max_lifetime":  &h.SSEMaxLifetime,
	}
	vals := map[string]string{
		"request_timeout":   reqStr,
		"history_timeout":   histStr,
		"sse_ping_interval": pingStr,
		"sse_max_lifetime":  maxLifeStr,
	}
	for key, durPtr := range pairs {
		if text := vals[key]; text != "" {
			*durPtr, err = time.ParseDuration(text)
			if err != nil {
				return fmt.Errorf("invalid %s: %w", key, err)
			}
		}
	}
	return nil
}

func ParsePort(text string, fallback int) int {
	value, err := strconv.Atoi(text)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
