package config

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all application configuration
type Config struct {
	Download      DownloadConfig      `mapstructure:"download"`
	Player        PlayerConfig        `mapstructure:"player"`
	Search        SearchConfig        `mapstructure:"search"`
	Network       NetworkConfig       `mapstructure:"network"`
	Notifications NotificationsConfig `mapstructure:"notifications"`
}

// DownloadConfig holds download settings
type DownloadConfig struct {
	Directory       string `mapstructure:"directory"`
	ChunkSize       int64  `mapstructure:"chunk_size"`
	MaxConcurrent   int    `mapstructure:"max_concurrent"`
	PreferredFormat string `mapstructure:"preferred_format"`
}

// PlayerConfig holds audio player settings
type PlayerConfig struct {
	DefaultSpeed      float64 `mapstructure:"default_speed"`
	SkipSeconds       int     `mapstructure:"skip_seconds"`
	SleepTimerMinutes int     `mapstructure:"sleep_timer_minutes"`
}

// SearchConfig holds search settings
type SearchConfig struct {
	DefaultLimit int           `mapstructure:"default_limit"`
	CacheTTL     time.Duration `mapstructure:"cache_ttl"`
	Sources      []string      `mapstructure:"sources"`
}

// NetworkConfig holds network settings
type NetworkConfig struct {
	Timeout       time.Duration `mapstructure:"timeout"`
	RetryAttempts int           `mapstructure:"retry_attempts"`
	UserAgent     string        `mapstructure:"user_agent"`
}

// NotificationsConfig holds notification settings
type NotificationsConfig struct {
	Enabled bool `mapstructure:"enabled"`
	Sound   bool `mapstructure:"sound"`
}

var cfg *Config

// GetConfigDir returns the configuration directory path
func GetConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "audbookdl")
}

// GetDBPath returns the database file path
func GetDBPath() string {
	return filepath.Join(GetConfigDir(), "audbookdl.db")
}

// GetConfigPath returns the config file path
func GetConfigPath() string {
	return filepath.Join(GetConfigDir(), "config.yaml")
}

// Init initializes the configuration with defaults
func Init(cfgFile string) error {
	// Reset viper state
	viper.Reset()

	// Set defaults
	viper.SetDefault("download.directory", "~/Audiobooks")
	viper.SetDefault("download.chunk_size", int64(5*1024*1024)) // 5MB
	viper.SetDefault("download.max_concurrent", 3)
	viper.SetDefault("download.preferred_format", "mp3")

	viper.SetDefault("player.default_speed", 1.0)
	viper.SetDefault("player.skip_seconds", 15)
	viper.SetDefault("player.sleep_timer_minutes", 0)

	viper.SetDefault("search.default_limit", 10)
	viper.SetDefault("search.cache_ttl", 1*time.Hour)
	viper.SetDefault("search.sources", []string{"librivox", "archive", "loyalbooks", "openlibrary"})

	viper.SetDefault("network.timeout", 30*time.Second)
	viper.SetDefault("network.retry_attempts", 3)
	viper.SetDefault("network.user_agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	viper.SetDefault("notifications.enabled", false)
	viper.SetDefault("notifications.sound", false)

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(GetConfigDir())
	}

	// Environment variable overrides
	viper.SetEnvPrefix("AUDBOOKDL")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Read config file (ignore if not found)
	_ = viper.ReadInConfig()

	// Reset cached config so Get() re-reads
	cfg = nil

	return nil
}

// Get returns the current configuration (cached)
func Get() *Config {
	if cfg == nil {
		cfg = &Config{}
		_ = viper.Unmarshal(cfg)
		cfg.Download.Directory = expandPath(cfg.Download.Directory)
	}
	return cfg
}

// Set sets a configuration value and writes it to disk
func Set(key, value string) error {
	viper.Set(key, value)

	// Ensure config directory exists
	configDir := GetConfigDir()
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	// Reset cached config
	cfg = nil

	return viper.WriteConfigAs(GetConfigPath())
}

// GetValue retrieves a configuration value by key
func GetValue(key string) interface{} {
	return viper.Get(key)
}

// expandPath expands a leading ~/ to the user's home directory
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

// resetForTest resets viper and the cached config (used in tests)
func resetForTest() {
	viper.Reset()
	cfg = nil
}
