package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	App       AppConfig       `mapstructure:"app"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Redis     RedisConfig     `mapstructure:"redis"`
	Log       LogConfig       `mapstructure:"log"`
	Cache     CacheConfig     `mapstructure:"cache"`
	Queue     QueueConfig     `mapstructure:"queue"`
	CORS      CORSConfig      `mapstructure:"cors"`
	SMTP      SMTPConfig      `mapstructure:"smtp"`
	Telegram  TelegramConfig  `mapstructure:"telegram"`
	Plugins   PluginsConfig   `mapstructure:"plugins"`
	RateLimit RateLimitConfig `mapstructure:"rate_limit"`
}

type ServerConfig struct {
	Mode           string        `mapstructure:"mode"`
	Port           int           `mapstructure:"port"`
	ReadTimeout    time.Duration `mapstructure:"read_timeout"`
	WriteTimeout   time.Duration `mapstructure:"write_timeout"`
	MaxHeaderBytes int           `mapstructure:"max_header_bytes"`
}

type AppConfig struct {
	Name                string   `mapstructure:"name"`
	Version             string   `mapstructure:"version"`
	Key                 string   `mapstructure:"key"`
	Debug               bool     `mapstructure:"debug"`
	URL                 string   `mapstructure:"url"`
	SecurePath          string   `mapstructure:"secure_path"`
	SubscribePath       string   `mapstructure:"subscribe_path"`
	InviteForce         bool     `mapstructure:"invite_force"`
	InviteCommission    int      `mapstructure:"invite_commission"`
	TryOutPlan          string   `mapstructure:"try_out_plan"`
	TryOutHours         int      `mapstructure:"try_out_hours"`
	EmailWhitelist      []string `mapstructure:"email_whitelist"`
	EmailWhitelistEnable bool    `mapstructure:"email_whitelist_enabled"`
	StopRegister        bool     `mapstructure:"stop_register"`
	EnableAutoBackup    bool     `mapstructure:"enable_auto_backup"`
	BackupInterval      int      `mapstructure:"backup_interval"`
	TelegramBotToken    string   `mapstructure:"telegram_bot_token"`
}

type DatabaseConfig struct {
	Driver         string        `mapstructure:"driver"`
	Host           string        `mapstructure:"host"`
	Port           int           `mapstructure:"port"`
	DBName         string        `mapstructure:"dbname"`
	Username       string        `mapstructure:"username"`
	Password       string        `mapstructure:"password"`
	Charset        string        `mapstructure:"charset"`
	Collation      string        `mapstructure:"collation"`
	MaxIdleConns   int           `mapstructure:"max_idle_conns"`
	MaxOpenConns   int           `mapstructure:"max_open_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	TablePrefix    string        `mapstructure:"table_prefix"`
	SQLitePath     string        `mapstructure:"sqlite_path"`
}

func (d *DatabaseConfig) DSN() string {
	return d.Username + ":" + d.Password + "@tcp(" + d.Host + ":" + itoa(d.Port) + ")/" +
		d.DBName + "?charset=" + d.Charset + "&collation=" + d.Collation + "&parseTime=true&loc=Local"
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	s := ""
	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}
	return s
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	Prefix   string `mapstructure:"prefix"`
}

func (r *RedisConfig) Addr() string {
	return r.Host + ":" + itoa(r.Port)
}

type LogConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	Output     string `mapstructure:"output"`
	File       string `mapstructure:"file"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
	Compress   bool   `mapstructure:"compress"`
}

type CacheConfig struct {
	Driver string `mapstructure:"driver"`
	TTL    int    `mapstructure:"ttl"`
}

type QueueConfig struct {
	Driver      string        `mapstructure:"driver"`
	MaxAttempts int           `mapstructure:"max_attempts"`
	RetryDelay  time.Duration `mapstructure:"retry_delay"`
}

type CORSConfig struct {
	AllowedOrigins []string `mapstructure:"allowed_origins"`
	AllowedMethods []string `mapstructure:"allowed_methods"`
	AllowedHeaders []string `mapstructure:"allowed_headers"`
	MaxAge         int      `mapstructure:"max_age"`
}

type SMTPConfig struct {
	Host        string `mapstructure:"host"`
	Port        int    `mapstructure:"port"`
	Username    string `mapstructure:"username"`
	Password    string `mapstructure:"password"`
	Encryption  string `mapstructure:"encryption"` // tls | ssl
	FromAddress string `mapstructure:"from_address"`
	FromName    string `mapstructure:"from_name"`
}

type TelegramConfig struct {
	BotToken string `mapstructure:"bot_token"`
	Proxy    string `mapstructure:"proxy"`
}

type PluginsConfig struct {
	Path         string `mapstructure:"path"`
	CorePath     string `mapstructure:"core_path"`
	AutoInstall  bool   `mapstructure:"auto_install"`
}

type RateLimitConfig struct {
	Enabled  bool          `mapstructure:"enabled"`
	Requests int           `mapstructure:"requests"`
	Duration time.Duration `mapstructure:"duration"`
}

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetEnvPrefix("XBOARD")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
