package rocketmq

import (
	"context"
	"strings"
	"sync"
	"time"

	rmq "github.com/apache/rocketmq-clients/golang/v5"
	"github.com/apache/rocketmq-clients/golang/v5/credentials"
	beego "github.com/beego/beego/v2/server/web"

	"dt-server/common/logger"

	"go.uber.org/zap"
)

// Publisher is a minimal facade for sending messages.
type Publisher interface {
	Publish(topic string, body []byte) error
}

// Consumer placeholder retained for future extension.
type Consumer interface {
	Start() error
	Stop() error
}

var (
	initOnce sync.Once
	enabled  bool
	prod     rmq.Producer
	pub      Publisher
)

// Enabled reports whether MQ is configured and producer started.
func Enabled() bool { initOnce.Do(initMQ); return enabled }

// PublisherInstance returns the active publisher (stub if disabled).
func PublisherInstance() Publisher {
	initOnce.Do(initMQ)
	if pub == nil {
		pub = &stubPublisher{}
	}
	return pub
}

// Real publisher backed by RocketMQ v5 client.
type rmqPublisher struct{ p rmq.Producer }

func (r *rmqPublisher) Publish(topic string, body []byte) error {
	if r.p == nil {
		return nil
	}
	msg := &rmq.Message{Topic: topic, Body: body}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := r.p.Send(ctx, msg)
	return err
}

// Stub publisher used when MQ is disabled.
type stubPublisher struct{}

func (s *stubPublisher) Publish(topic string, body []byte) error {
	logger.Warn("[mq disabled] drop message", zap.String("topic", topic))
	return nil
}

func initMQ() {
	// Use SDK's ResetLogger to avoid default file-based logging under /logs
	rmq.ResetLogger()

	endpoint, _ := beego.AppConfig.String("rocketmq_endpoint")
	if endpoint == "" {
		// backward compatibility for earlier config key
		endpoint, _ = beego.AppConfig.String("rocketmq_namesrv")
	}
	if endpoint == "" {
		enabled = false
		pub = &stubPublisher{}
		return
	}
	// sanitize endpoint: trim, strip scheme, pick first if contains ',' or ';'
	endpoint = strings.TrimSpace(endpoint)
	endpoint = strings.TrimPrefix(strings.TrimPrefix(endpoint, "http://"), "https://")
	if idx := strings.IndexAny(endpoint, ",;"); idx > 0 {
		endpoint = strings.TrimSpace(endpoint[:idx])
	}
	ak, _ := beego.AppConfig.String("rocketmq_access_key")
	sk, _ := beego.AppConfig.String("rocketmq_secret_key")
	topicsStr, _ := beego.AppConfig.String("rocketmq_producer_topics")

	// 安全起见：若缺少凭证则禁用 MQ（避免底层 SDK 在 Sign 阶段空指针崩溃）
	if strings.TrimSpace(ak) == "" || strings.TrimSpace(sk) == "" {
		enabled = false
		pub = &stubPublisher{}
		logger.Warn("rocketmq disabled: missing access/secret key while endpoint present")
		return
	}

	cfg := &rmq.Config{Endpoint: endpoint}
	cfg.Credentials = &credentials.SessionCredentials{AccessKey: ak, AccessSecret: sk}
	logger.Info("rocketmq producer config", zap.String("endpoint", endpoint), zap.String("topics", topicsStr), zap.String("ak", ak))

	var opts []rmq.ProducerOption
	if topicsStr != "" {
		parts := strings.Split(topicsStr, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(strings.ReplaceAll(parts[i], ".", "_"))
		}
		opts = append(opts, rmq.WithTopics(parts...))
		logger.Info("rocketmq: topics configured", zap.Strings("topics", parts))
	}

	logger.Info("rocketmq: creating producer", zap.Int("opts_count", len(opts)))
	p, err := rmq.NewProducer(cfg, opts...)
	if err != nil {
		logger.Error("rocketmq: producer init failed", zap.Error(err))
		enabled = false
		pub = &stubPublisher{}
		return
	}

	logger.Info("rocketmq: producer created, starting (this may take a few seconds)...")

	// 使用 goroutine 异步启动，避免阻塞主流程
	startDone := make(chan error, 1)
	go func() {
		startDone <- p.Start()
	}()

	// 等待启动完成或超时（2秒）
	select {
	case err := <-startDone:
		if err != nil {
			logger.Warn("rocketmq: producer start failed (will use stub publisher)", zap.Error(err))
			enabled = false
			pub = &stubPublisher{}
			return
		}
		prod = p
		pub = &rmqPublisher{p: p}
		enabled = true
		logger.Info("rocketmq enabled", zap.String("endpoint", endpoint))
	case <-time.After(2 * time.Second):
		logger.Warn("rocketmq: producer start timeout (will use stub publisher, messages will be dropped)")
		enabled = false
		pub = &stubPublisher{}
		return
	}
}
