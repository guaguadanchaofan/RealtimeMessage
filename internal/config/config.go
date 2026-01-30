package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Runtime  RuntimeConfig  `yaml:"runtime"`
	Network  NetworkConfig  `yaml:"network"`
	Redis    RedisConfig    `yaml:"redis"`
	Dingding DingdingConfig `yaml:"dingding"`
	Scoring  ScoringConfig  `yaml:"scoring"`
	Sources  []SourceConfig `yaml:"sources"`
	Topics   []TopicConfig  `yaml:"topics"`
	Triggers TriggerConfig  `yaml:"triggers"`
	Push     PushConfig     `yaml:"push"`
	Dedupe   DedupeConfig   `yaml:"dedupe"`
	Logging  LoggingConfig  `yaml:"logging"`
}

type RuntimeConfig struct {
	Timezone                   string `yaml:"timezone"`
	DefaultPollIntervalSeconds int    `yaml:"default_poll_interval_seconds"`
	ReloadIntervalSeconds      int    `yaml:"reload_interval_seconds"`
}

type NetworkConfig struct {
	DefaultTimeoutMS int        `yaml:"default_timeout_ms"`
	Retry            RetryConfig `yaml:"retry"`
}

type RetryConfig struct {
	MaxAttempts  int     `yaml:"max_attempts"`
	BackoffMS    int     `yaml:"backoff_ms"`
	Multiplier   float64 `yaml:"multiplier"`
	JitterMS     int     `yaml:"jitter_ms"`
	RetryOnStatus []int  `yaml:"retry_on_status"`
}

type RedisConfig struct {
	Addr      string `yaml:"addr"`
	Password  string `yaml:"password"`
	DB        int    `yaml:"db"`
	KeyPrefix string `yaml:"key_prefix"`
}

type DingdingConfig struct {
	Webhook  string `yaml:"webhook"`
	Secret   string `yaml:"secret"`
	MsgType  string `yaml:"msg_type"`
	Title    string `yaml:"title"`
	TimeoutMS int   `yaml:"timeout_ms"`
}

type ScoringConfig struct {
	PushThreshold int           `yaml:"push_threshold"`
	MarketHours   MarketHoursConfig `yaml:"market_hours"`
}

type MarketHoursConfig struct {
	Enabled          bool `yaml:"enabled"`
	InSessionBonus   int  `yaml:"in_session_bonus"`
	OffSessionPenalty int `yaml:"off_session_penalty"`
}

type SourceConfig struct {
	Name                 string        `yaml:"name"`
	Type                 string        `yaml:"type"`
	URL                  string        `yaml:"url"`
	PollIntervalSeconds  int           `yaml:"poll_interval_seconds"`
	TimeoutMS            int           `yaml:"timeout_ms"`
	Retry                RetryConfig    `yaml:"retry"`
	BaseScore            int           `yaml:"base_score"`
	Headers              map[string]string `yaml:"headers"`
	Parser               ParserConfig   `yaml:"parser"`
}

type ParserConfig struct {
	Mode    string        `yaml:"mode"`
	Mapping MappingConfig `yaml:"mapping"`
}

type MappingConfig struct {
	ListPath string            `yaml:"list_path"`
	Fields   map[string]string `yaml:"fields"`
}

type TopicConfig struct {
	Name     string   `yaml:"name"`
	Weight   int      `yaml:"weight"`
	Keywords []string `yaml:"keywords"`
}

type TriggerConfig struct {
	Strong StrongTriggerConfig `yaml:"strong"`
}

type StrongTriggerConfig struct {
	Weight   int      `yaml:"weight"`
	Keywords []string `yaml:"keywords"`
}

type PushConfig struct {
	MaxPushPerMinute int          `yaml:"max_push_per_minute"`
	Template         TemplateConfig `yaml:"template"`
}

type TemplateConfig struct {
	Markdown string `yaml:"markdown"`
}

type DedupeConfig struct {
	TTLHours   int      `yaml:"ttl_hours"`
	KeyStrategy []string `yaml:"key_strategy"`
}

type LoggingConfig struct {
	Level string `yaml:"level"`
	JSON  bool   `yaml:"json"`
}

func Load(path string) (Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	expanded := os.ExpandEnv(string(raw))
	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return Config{}, err
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if len(c.Sources) == 0 {
		return errors.New("no sources configured")
	}
	if c.Runtime.DefaultPollIntervalSeconds <= 0 {
		return errors.New("runtime.default_poll_interval_seconds must be > 0")
	}
	if c.Network.DefaultTimeoutMS <= 0 {
		return errors.New("network.default_timeout_ms must be > 0")
	}
	if c.Network.Retry.MaxAttempts <= 0 {
		return errors.New("network.retry.max_attempts must be > 0")
	}
	for i, src := range c.Sources {
		if strings.TrimSpace(src.Name) == "" {
			return fmt.Errorf("sources[%d].name required", i)
		}
		if strings.TrimSpace(src.Type) == "" {
			return fmt.Errorf("sources[%d].type required", i)
		}
		if strings.TrimSpace(src.URL) == "" {
			return fmt.Errorf("sources[%d].url required", i)
		}
	}
	return nil
}
