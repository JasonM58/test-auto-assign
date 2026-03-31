package config

import (
	"log/slog"
	"os"
	"strings"
	"sync"

	"gopkg.in/yaml.v2"
)

type (
	Loader struct {
		configFilePath string
	}
)

var (
	configOnce   sync.Once
	loaderVar    Loader
	fileLoadOnce sync.Once
	loadedConfig Config
	loadErr      error
)

// NewLoader initializes the config loader singleton.
func NewLoader(configFilePath string) *Loader {
	configOnce.Do(func() {
		loaderVar = Loader{configFilePath: configFilePath}
	})
	return &loaderVar
}

func (l *Loader) fileLoad() Config {
	fileLoadOnce.Do(func() {
		f, err := os.ReadFile(l.configFilePath)
		if err != nil {
			loadErr = err
			slog.Error("failed to read config file", slog.Any("error", err))
			return
		}

		if err := yaml.Unmarshal(f, &loadedConfig); err != nil {
			loadErr = err
			slog.Error("failed to unmarshal config file", slog.Any("error", err))
			return
		}
	})
	return loadedConfig
}

func (l *Loader) Reload() error {
	f, err := os.ReadFile(l.configFilePath)
	if err != nil {
		return err
	}

	newConfig := Config{}
	if err := yaml.Unmarshal(f, &newConfig); err != nil {
		return err
	}

	loadedConfig = newConfig
	loadErr = nil
	slog.Info("config reloaded successfully", slog.String("path", l.configFilePath))
	return nil
}

// --- GITHUB CONFIG ---
func (l *Loader) GetGithubAppId() int64 { return l.fileLoad().Github.AppId }
func (l *Loader) GetGithubPrivateKeyPath() string {
	return l.fileLoad().Github.PrivateKeyPath
}
func (l *Loader) GetGithubPrivateKey() string { return l.fileLoad().Github.PrivateKey }

// --- LARK CONFIG ---
func (l *Loader) GetLarkSecret() string { return l.fileLoad().Lark.Secret }
func (l *Loader) GetLarkAppID() string  { return l.fileLoad().Lark.AppId }
func (l *Loader) GetLarkAppSecret() string {
	return l.fileLoad().Lark.AppSecret
}
func (l *Loader) GetLarkWebhookURL() string {
	return l.fileLoad().Lark.WebHookUrl
}
func (l *Loader) GetLarkGithubToEmailMap() map[string]string {
	raw := strings.TrimSpace(l.fileLoad().Lark.GithubToEmailMap)
	if raw == "" {
		return nil
	}

	result := make(map[string]string)
	pairs := strings.Split(raw, ",")
	for _, p := range pairs {
		kv := strings.SplitN(strings.TrimSpace(p), "=", 2)
		if len(kv) == 2 {
			result[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return result
}

// --- TELEMETRY CONFIG ---
func (l *Loader) GetTelemetryEnabled() bool {
	return l.fileLoad().Telemetry.Enabled
}
func (l *Loader) GetOTLPEndpoint() string {
	t := l.fileLoad().Telemetry
	if t.OTLPEndpoint == "" {
		return "localhost:4317"
	}
	return t.OTLPEndpoint
}
func (l *Loader) GetOTLPInsecure() bool {
	t := l.fileLoad().Telemetry
	return t.OTLPInsecure || t.OTLPEndpoint == ""
}
func (l *Loader) GetEnvironment() string {
	env := l.fileLoad().Telemetry.Environment
	if env == "" {
		return "dev"
	}
	return env
}
func (l *Loader) GetTelemetryMetricExportInterval() int {
	val := l.fileLoad().Telemetry.MetricExportInterval
	if val == 0 {
		return 10
	}
	return val
}

// --- PROMETHEUS CONFIG ---
func (l *Loader) GetPrometheusURL() string {
	u := l.fileLoad().Prometheus.BaseURL
	if u == "" {
		return ""
	}
	return u
}
func (l *Loader) GetPrometheusQueryTimeoutSecs() int {
	return l.fileLoad().Prometheus.QueryTimeoutSecs
}
func (l *Loader) GetPrometheusMaxRetries() int {
	return l.fileLoad().Prometheus.MaxRetries
}
func (l *Loader) GetPrometheusRetryBaseDelayMs() int {
	return l.fileLoad().Prometheus.RetryBaseDelayMs
}
