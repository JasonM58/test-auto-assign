package config

import (
	"log/slog"
	"os"
	"strings"
	"sync"

	"gopkg.in/yaml.v2"
)

type (
	cfg struct {
		configFilePath string
	}
)

var (
	configOnce    sync.Once
	configVar     cfg
	fileLoadOnce  sync.Once
	config        Config
	loadErr       error
)

// NewConfig initializes config singleton
func NewConfig(configFilePath string) *cfg {
	configOnce.Do(func() {
		configVar = cfg{configFilePath: configFilePath}
	})
	return &configVar
}

// fileLoad reads the YAML file once and caches it
func (c *cfg) fileLoad() Config {
	fileLoadOnce.Do(func() {
		f, err := os.ReadFile(c.configFilePath)
		if err != nil {
			loadErr = err
			slog.Error("failed to read config file", slog.Any("error", err))
			return
		}

		if err := yaml.Unmarshal(f, &config); err != nil {
			loadErr = err
			slog.Error("failed to unmarshal config file", slog.Any("error", err))
			return
		}
	})
	return config
}

// Reload re-reads the config file — useful for local/dev hot reload
func (c *cfg) Reload() error {
	f, err := os.ReadFile(c.configFilePath)
	if err != nil {
		return err
	}

	newConfig := Config{}
	if err := yaml.Unmarshal(f, &newConfig); err != nil {
		return err
	}

	config = newConfig
	loadErr = nil
	slog.Info("config reloaded successfully", slog.String("path", c.configFilePath))
	return nil
}

// --- GITHUB CONFIG ---

func (c *cfg) GetGithubAppId() int64 {
	loaded := c.fileLoad()
	return loaded.Github.AppId
}

func (c *cfg) GetGithubPrivateKeyPath() string {
	loaded := c.fileLoad()
	return loaded.Github.PrivateKeyPath
}

func (c *cfg) GetGithubPrivateKey() string {
	loaded := c.fileLoad()
	return loaded.Github.PrivateKey
}

// --- LARK CONFIG ---

func (c *cfg) GetLarkSecret() string {
	loaded := c.fileLoad()
	return loaded.Lark.Secret
}

func (c *cfg) GetLarkAppID() string {
	loaded := c.fileLoad()
	return loaded.Lark.AppId
}

func (c *cfg) GetLarkAppSecret() string {
	loaded := c.fileLoad()
	return loaded.Lark.AppSecret
}

func (c *cfg) GetLarkWebhookURL() string {
	loaded := c.fileLoad()
	return loaded.Lark.WebHookUrl
}

func (c *cfg) GetLarkGithubToEmailMap() map[string]string {
    loaded := c.fileLoad()
    raw := strings.TrimSpace(loaded.Lark.GithubToEmailMap)
    if raw == "" {
        return nil
    }

    result := make(map[string]string)
    pairs := strings.Split(raw, ",")
    for _, p := range pairs {
        kv := strings.SplitN(strings.TrimSpace(p), "=", 2)
        if len(kv) == 2 {
            key := strings.TrimSpace(kv[0])
            val := strings.TrimSpace(kv[1])
            result[key] = val
        }
    }
    return result
}


// --- TELEMETRY CONFIG ---

func (c *cfg) GetTelemetryEnabled() bool {
	loaded := c.fileLoad()
	return loaded.Telemetry.Enabled
}

func (c *cfg) GetOTLPEndpoint() string {
	loaded := c.fileLoad()
	if loaded.Telemetry.OTLPEndpoint == "" {
		return "localhost:4317"
	}
	return loaded.Telemetry.OTLPEndpoint
}

func (c *cfg) GetOTLPInsecure() bool {
	loaded := c.fileLoad()
	return loaded.Telemetry.OTLPInsecure || loaded.Telemetry.OTLPEndpoint == ""
}

func (c *cfg) GetEnvironment() string {
	loaded := c.fileLoad()
	if loaded.Telemetry.Environment == "" {
		return "dev"
	}
	return loaded.Telemetry.Environment
}

func (c *cfg) GetTelemetryMetricExportInterval() int {
	loaded := c.fileLoad()
	if loaded.Telemetry.MetricExportInterval == 0 {
		return 10 // default
	}
	return loaded.Telemetry.MetricExportInterval
}
