package config

import (
	"sync/atomic"
)

// 原子存储当前生效的配置，供各业务读取
var current atomic.Value // *Config

func SetCurrent(c *Config) {
	current.Store(c)
}

func GetCurrent() *Config {
	v := current.Load()
	if v == nil {
		return nil
	}
	return v.(*Config)
}

// GetFeatureFlag 返回功能开关（默认 false）
func GetFeatureFlag(name string) bool {
	cfg := GetCurrent()
	if cfg == nil || cfg.FeatureFlags == nil {
		return false
	}
	return cfg.FeatureFlags[name]
}

// GetThreshold 返回业务阈值（支持默认值）
func GetThreshold(name string, def int64) int64 {
	cfg := GetCurrent()
	if cfg == nil || cfg.Thresholds == nil {
		return def
	}
	if v, ok := cfg.Thresholds[name]; ok {
		return v
	}
	return def
}

