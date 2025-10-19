package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	clientv3 "go.etcd.io/etcd/client/v3"
	"gopkg.in/yaml.v3"
)

// Config 与 Etcd 中的配置结构对应
// 注意：时间字段统一使用毫秒时间戳
// 可按需扩展字段

type Config struct {
	Server struct {
		Port     int    `yaml:"port" json:"port"`
		LogLevel string `yaml:"log_level" json:"log_level"`
	} `yaml:"server" json:"server"`

	Database struct {
		DSN                string `yaml:"dsn" json:"dsn"`
		MaxOpenConns       int    `yaml:"max_open_conns" json:"max_open_conns"`
		MaxIdleConns       int    `yaml:"max_idle_conns" json:"max_idle_conns"`
		ConnMaxLifetimeSec int    `yaml:"conn_max_lifetime_sec" json:"conn_max_lifetime_sec"`
	} `yaml:"database" json:"database"`

	Redis struct {
		Addr     string `yaml:"addr" json:"addr"`
		Password string `yaml:"password" json:"password"`
		DB       int    `yaml:"db" json:"db"`
	} `yaml:"redis" json:"redis"`

	RocketMQ struct {
		NameServer    string `yaml:"name_server" json:"name_server"`
		ProducerGroup string `yaml:"producer_group" json:"producer_group"`
		ConsumerGroup string `yaml:"consumer_group" json:"consumer_group"`
		TopicSettle   string `yaml:"topic_settle" json:"topic_settle"`
		AccessKey     string `yaml:"access_key" json:"access_key"`
		SecretKey     string `yaml:"secret_key" json:"secret_key"`
	} `yaml:"rocketmq" json:"rocketmq"`

	Observability struct {
		EnableProm   bool   `yaml:"enable_prom" json:"enable_prom"`
		PromAddr     string `yaml:"prom_addr" json:"prom_addr"`
		EnableTrace  bool   `yaml:"enable_trace" json:"enable_trace"`
		OtlpEndpoint string `yaml:"otlp_endpoint" json:"otlp_endpoint"`
	} `yaml:"observability" json:"observability"`

	Auth struct {
		DemoMode bool `yaml:"demo_mode" json:"demo_mode"` // 演示模式开关
		JWT      struct {
			Secret          string `yaml:"secret" json:"secret"`
			AccessTokenTTL  int    `yaml:"access_token_ttl" json:"access_token_ttl"`   // 秒
			RefreshTokenTTL int    `yaml:"refresh_token_ttl" json:"refresh_token_ttl"` // 秒
			Issuer          string `yaml:"issuer" json:"issuer"`
		} `yaml:"jwt" json:"jwt"`
		Admin struct {
			Enabled bool   `yaml:"enabled" json:"enabled"`
			Token   string `yaml:"token" json:"token"`
		} `yaml:"admin" json:"admin"`
		DemoPlatform struct {
			PlatformID int8   `yaml:"platform_id" json:"platform_id"`
			AppKey     string `yaml:"app_key" json:"app_key"`
			AppSecret  string `yaml:"app_secret" json:"app_secret"`
			Name       string `yaml:"name" json:"name"`
		} `yaml:"demo_platform" json:"demo_platform"`
		Platforms []PlatformConfig `yaml:"platforms" json:"platforms"`
	} `yaml:"auth" json:"auth"`

	RateLimit struct {
		Enabled bool `yaml:"enabled" json:"enabled"`
		Global  struct {
			RequestsPerSecond int `yaml:"requests_per_second" json:"requests_per_second"`
			Burst             int `yaml:"burst" json:"burst"`
		} `yaml:"global" json:"global"`
		ByIP struct {
			RequestsPerSecond int `yaml:"requests_per_second" json:"requests_per_second"`
			Burst             int `yaml:"burst" json:"burst"`
			WindowSeconds     int `yaml:"window_seconds" json:"window_seconds"`
		} `yaml:"by_ip" json:"by_ip"`
		ByUser struct {
			RequestsPerSecond int `yaml:"requests_per_second" json:"requests_per_second"`
			Burst             int `yaml:"burst" json:"burst"`
			WindowSeconds     int `yaml:"window_seconds" json:"window_seconds"`
		} `yaml:"by_user" json:"by_user"`
		ByPlatform struct {
			RequestsPerSecond int `yaml:"requests_per_second" json:"requests_per_second"`
			Burst             int `yaml:"burst" json:"burst"`
			WindowSeconds     int `yaml:"window_seconds" json:"window_seconds"`
		} `yaml:"by_platform" json:"by_platform"`
	} `yaml:"rate_limit" json:"rate_limit"`

	CORS struct {
		Enabled          bool     `yaml:"enabled" json:"enabled"`
		AllowedOrigins   []string `yaml:"allowed_origins" json:"allowed_origins"`
		AllowedMethods   []string `yaml:"allowed_methods" json:"allowed_methods"`
		AllowedHeaders   []string `yaml:"allowed_headers" json:"allowed_headers"`
		ExposedHeaders   []string `yaml:"exposed_headers" json:"exposed_headers"`
		AllowCredentials bool     `yaml:"allow_credentials" json:"allow_credentials"`
		MaxAge           int      `yaml:"max_age" json:"max_age"`
	} `yaml:"cors" json:"cors"`

	// 第一步动态配置：功能开关与业务阈值
	FeatureFlags map[string]bool  `yaml:"feature_flags" json:"feature_flags"`
	Thresholds   map[string]int64 `yaml:"thresholds" json:"thresholds"`
}

// PlatformConfig 平台配置
type PlatformConfig struct {
	PlatformID int8     `yaml:"platform_id" json:"platform_id"`
	AppKey     string   `yaml:"app_key" json:"app_key"`
	AppSecret  string   `yaml:"app_secret" json:"app_secret"`
	Name       string   `yaml:"name" json:"name"`
	Status     int8     `yaml:"status" json:"status"`
	RateLimit  int      `yaml:"rate_limit" json:"rate_limit"`
	AllowedIPs []string `yaml:"allowed_ips" json:"allowed_ips"`
}

// Load 优先从 Nacos 配置中心读取配置，如果失败则从本地文件读取（兜底）
// 支持以下环境变量：
//   - NACOS_SERVER_ADDR: Nacos 服务器地址（如 "127.0.0.1:8848"，如果设置则优先从 Nacos 加载）
//   - NACOS_DATA_ID: 配置 Data ID（如 "dt-server.yaml"）
//   - NACOS_NAMESPACE: 命名空间 ID（可选，默认 public）
//   - NACOS_GROUP: 配置分组（可选，默认 DEFAULT_GROUP）
//   - CONFIG_FILE: 配置文件路径（兜底方案，默认：config/dev.json）
func Load(ctx context.Context) (*Config, error) {
	// 1. 优先尝试从 Nacos 加载
	nacosServerAddr := strings.TrimSpace(os.Getenv("NACOS_SERVER_ADDR"))
	if nacosServerAddr != "" {
		cfg, err := loadFromNacos(ctx)
		if err == nil {
			fmt.Printf("[Config]  配置已从 Nacos 加载: server=%s, dataId=%s, namespace=%s, group=%s\n",
				nacosServerAddr,
				os.Getenv("NACOS_DATA_ID"),
				getEnvOrDefault("NACOS_NAMESPACE", "public"),
				getEnvOrDefault("NACOS_GROUP", "DEFAULT_GROUP"))
			return cfg, nil
		}
		// Nacos 加载失败，记录错误并降级到本地文件
		fmt.Printf("[Config]  从 Nacos 加载配置失败，降级使用本地文件: error=%v\n", err)
	}

	// 2. 降级：从本地文件加载
	configFile := os.Getenv("CONFIG_FILE")
	if configFile == "" {
		configFile = "config/dev.json"
	}

	cfg, err := loadFromFile(configFile)
	if err == nil {
		fmt.Printf("[Config] 配置已从本地文件加载: file=%s\n", configFile)
		return cfg, nil
	}

	// 3. 两种方式都失败，返回错误
	return nil, fmt.Errorf("failed to load config from nacos and local file (%s): %w", configFile, err)
}

// getEnvOrDefault 获取环境变量，如果不存在则返回默认值
func getEnvOrDefault(key, defaultValue string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return defaultValue
}

// loadFromFile 从本地 JSON 或 YAML 文件加载配置
func loadFromFile(filePath string) (*Config, error) {
	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", filePath)
	}

	// 读取文件内容
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config

	// 根据文件扩展名选择解析方式
	ext := filepath.Ext(filePath)
	switch ext {
	case ".json":
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported config file format: %s (supported: .json, .yaml, .yml)", ext)
	}

	return &cfg, nil
}

func loadFromEtcd(ctx context.Context) (*Config, error) {
	endpoints := strings.Split(os.Getenv("ETCD_ENDPOINTS"), ",")
	for i := range endpoints {
		endpoints[i] = strings.TrimSpace(endpoints[i])
	}
	if len(endpoints) == 0 || endpoints[0] == "" {
		return nil, errors.New("empty ETCD_ENDPOINTS")
	}
	dialTimeout := 5 * time.Second
	if v := os.Getenv("ETCD_DIAL_TIMEOUT_SEC"); strings.TrimSpace(v) != "" {
		if sec, err := time.ParseDuration(v + "s"); err == nil {
			dialTimeout = sec
		}
	}
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: dialTimeout,
		Username:    os.Getenv("ETCD_USERNAME"),
		Password:    os.Getenv("ETCD_PASSWORD"),
	})
	if err != nil {
		return nil, fmt.Errorf("etcd connect failed: %w", err)
	}
	defer cli.Close()

	key := os.Getenv("ETCD_CONFIG_KEY")
	if strings.TrimSpace(key) == "" {
		return nil, errors.New("ETCD_CONFIG_KEY not set")

	}
	ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := cli.Get(ctx2, key)
	if err != nil {
		return nil, fmt.Errorf("etcd get failed: %w", err)
	}
	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("etcd key not found: %s", key)
	}
	var cfg Config
	if err := yaml.Unmarshal(resp.Kvs[0].Value, &cfg); err != nil {
		return nil, fmt.Errorf("yaml unmarshal from etcd failed: %w", err)
	}
	return &cfg, nil
}

// loadFromNacos 从 Nacos 配置中心加载配置
// 支持以下环境变量：
//   - NACOS_SERVER_ADDR: Nacos 服务器地址（必填，如 "127.0.0.1:8848"）
//   - NACOS_NAMESPACE: 命名空间 ID（可选，默认为 public）
//   - NACOS_GROUP: 配置分组（可选，默认为 DEFAULT_GROUP）
//   - NACOS_DATA_ID: 配置 Data ID（必填，如 "dt-server.yaml"）
//   - NACOS_USERNAME: 用户名（可选）
//   - NACOS_PASSWORD: 密码（可选）
//   - NACOS_TIMEOUT_MS: 超时时间（毫秒，可选，默认 5000）
func loadFromNacos(ctx context.Context) (*Config, error) {
	// 1. 读取环境变量
	serverAddr := strings.TrimSpace(os.Getenv("NACOS_SERVER_ADDR"))
	if serverAddr == "" {
		return nil, errors.New("NACOS_SERVER_ADDR not set")
	}

	dataID := strings.TrimSpace(os.Getenv("NACOS_DATA_ID"))
	if dataID == "" {
		return nil, errors.New("NACOS_DATA_ID not set")
	}

	namespace := strings.TrimSpace(os.Getenv("NACOS_NAMESPACE"))
	if namespace == "" {
		namespace = "public" // 默认命名空间
	}

	group := strings.TrimSpace(os.Getenv("NACOS_GROUP"))
	if group == "" {
		group = "DEFAULT_GROUP" // 默认分组
	}

	username := strings.TrimSpace(os.Getenv("NACOS_USERNAME"))
	password := strings.TrimSpace(os.Getenv("NACOS_PASSWORD"))

	timeoutMS := 5000 // 默认 5 秒超时
	if timeoutStr := strings.TrimSpace(os.Getenv("NACOS_TIMEOUT_MS")); timeoutStr != "" {
		if t, err := strconv.Atoi(timeoutStr); err == nil && t > 0 {
			timeoutMS = t
		}
	}

	// 2. 解析服务器地址（支持多个地址，逗号分隔）
	serverAddrs := strings.Split(serverAddr, ",")
	var serverConfigs []constant.ServerConfig
	for _, addr := range serverAddrs {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		// 解析 host:port
		parts := strings.Split(addr, ":")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid NACOS_SERVER_ADDR format: %s (expected host:port)", addr)
		}
		host := parts[0]
		port, err := strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid port in NACOS_SERVER_ADDR: %s", parts[1])
		}
		serverConfigs = append(serverConfigs, constant.ServerConfig{
			IpAddr: host,
			Port:   port,
		})
	}

	if len(serverConfigs) == 0 {
		return nil, errors.New("no valid server address in NACOS_SERVER_ADDR")
	}

	// 3. 创建客户端配置
	clientConfig := constant.ClientConfig{
		NamespaceId:         namespace,
		TimeoutMs:           uint64(timeoutMS),
		NotLoadCacheAtStart: true,
		LogDir:              "/tmp/nacos/log",
		CacheDir:            "/tmp/nacos/cache",
		LogLevel:            "warn",
	}

	// 如果提供了用户名和密码，则启用认证
	if username != "" && password != "" {
		clientConfig.Username = username
		clientConfig.Password = password
	}

	// 4. 创建配置客户端
	configClient, err := clients.NewConfigClient(
		vo.NacosClientParam{
			ClientConfig:  &clientConfig,
			ServerConfigs: serverConfigs,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create nacos config client: %w", err)
	}

	// 5. 获取配置内容
	content, err := configClient.GetConfig(vo.ConfigParam{
		DataId: dataID,
		Group:  group,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get config from nacos: %w", err)
	}

	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("nacos config is empty: dataId=%s, group=%s", dataID, group)
	}

	// 6. 解析配置内容（支持 JSON 和 YAML）
	var cfg Config

	// 根据 Data ID 的扩展名判断格式
	ext := filepath.Ext(dataID)
	switch ext {
	case ".json":
		if err := json.Unmarshal([]byte(content), &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config from nacos: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal([]byte(content), &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config from nacos: %w", err)
		}
	default:
		// 默认尝试 YAML，如果失败再尝试 JSON
		if err := yaml.Unmarshal([]byte(content), &cfg); err != nil {
			if err2 := json.Unmarshal([]byte(content), &cfg); err2 != nil {
				return nil, fmt.Errorf("failed to parse config from nacos (tried YAML and JSON): yaml_err=%v, json_err=%v", err, err2)
			}
		}
	}

	return &cfg, nil
}

// nacosConfigClient 全局 Nacos 配置客户端，用于配置监听
var nacosConfigClient config_client.IConfigClient

// globalConfig 全局配置实例
var globalConfig *Config

// Set 设置全局配置
func Set(cfg *Config) {
	globalConfig = cfg
}

// Get 获取全局配置
func Get() *Config {
	return globalConfig
}
