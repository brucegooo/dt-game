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

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"gopkg.in/yaml.v3"
)

// StartWatch 监听配置变化，在变更时回调 onChange(old, new)
// 优先监听 Nacos 配置中心，如果 Nacos 未配置则跳过监听（使用本地文件配置时）
func StartWatch(ctx context.Context, onChange func(oldCfg, newCfg *Config)) error {
	// 检查是否配置了 Nacos
	nacosServerAddr := strings.TrimSpace(os.Getenv("NACOS_SERVER_ADDR"))
	if nacosServerAddr != "" {
		return startNacosWatch(ctx, onChange)
	}

	// Nacos 未配置，跳过监听（使用本地文件配置时）
	fmt.Println("[Config]  Nacos 未配置，跳过配置监听")
	return nil
}

// startNacosWatch 启动 Nacos 配置监听
func startNacosWatch(ctx context.Context, onChange func(oldCfg, newCfg *Config)) error {
	// 1. 读取环境变量
	serverAddr := strings.TrimSpace(os.Getenv("NACOS_SERVER_ADDR"))
	if serverAddr == "" {
		return errors.New("NACOS_SERVER_ADDR not set")
	}

	dataID := strings.TrimSpace(os.Getenv("NACOS_DATA_ID"))
	if dataID == "" {
		return errors.New("NACOS_DATA_ID not set")
	}

	namespace := strings.TrimSpace(os.Getenv("NACOS_NAMESPACE"))
	if namespace == "" {
		namespace = "public"
	}

	group := strings.TrimSpace(os.Getenv("NACOS_GROUP"))
	if group == "" {
		group = "DEFAULT_GROUP"
	}

	username := strings.TrimSpace(os.Getenv("NACOS_USERNAME"))
	password := strings.TrimSpace(os.Getenv("NACOS_PASSWORD"))

	timeoutMS := 5000
	if timeoutStr := strings.TrimSpace(os.Getenv("NACOS_TIMEOUT_MS")); timeoutStr != "" {
		if t, err := strconv.Atoi(timeoutStr); err == nil && t > 0 {
			timeoutMS = t
		}
	}

	// 2. 解析服务器地址
	serverAddrs := strings.Split(serverAddr, ",")
	var serverConfigs []constant.ServerConfig
	for _, addr := range serverAddrs {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		parts := strings.Split(addr, ":")
		if len(parts) != 2 {
			return fmt.Errorf("invalid NACOS_SERVER_ADDR format: %s", addr)
		}
		host := parts[0]
		port, err := strconv.ParseUint(parts[1], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid port in NACOS_SERVER_ADDR: %s", parts[1])
		}
		serverConfigs = append(serverConfigs, constant.ServerConfig{
			IpAddr: host,
			Port:   port,
		})
	}

	if len(serverConfigs) == 0 {
		return errors.New("no valid server address in NACOS_SERVER_ADDR")
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
		return fmt.Errorf("failed to create nacos config client for watch: %w", err)
	}

	// 保存全局客户端引用
	nacosConfigClient = configClient

	// 5. 启动监听
	err = configClient.ListenConfig(vo.ConfigParam{
		DataId: dataID,
		Group:  group,
		OnChange: func(namespace, group, dataId, data string) {
			fmt.Printf("[Config] 📡 Nacos 配置变更: namespace=%s, group=%s, dataId=%s\n",
				namespace, group, dataId)

			// 解析新配置
			var newCfg Config
			ext := filepath.Ext(dataId)
			var parseErr error

			switch ext {
			case ".json":
				parseErr = json.Unmarshal([]byte(data), &newCfg)
			case ".yaml", ".yml":
				parseErr = yaml.Unmarshal([]byte(data), &newCfg)
			default:
				// 默认尝试 YAML，失败再尝试 JSON
				parseErr = yaml.Unmarshal([]byte(data), &newCfg)
				if parseErr != nil {
					parseErr = json.Unmarshal([]byte(data), &newCfg)
				}
			}

			if parseErr != nil {
				fmt.Printf("[Config]  解析 Nacos 配置失败: error=%v\n", parseErr)
				return
			}

			// 更新配置并触发回调
			oldCfg := GetCurrent()
			SetCurrent(&newCfg)

			if onChange != nil {
				onChange(oldCfg, &newCfg)
			}

			fmt.Println("[Config]  Nacos 配置已更新")
		},
	})

	if err != nil {
		return fmt.Errorf("failed to listen nacos config: %w", err)
	}

	fmt.Printf("[Config]  Nacos 配置监听已启动: server=%s, dataId=%s, namespace=%s, group=%s\n",
		serverAddr, dataID, namespace, group)

	return nil
}
