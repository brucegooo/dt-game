package worker

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	rmq "github.com/apache/rocketmq-clients/golang/v5"
	"github.com/apache/rocketmq-clients/golang/v5/credentials"
	beego "github.com/beego/beego/v2/server/web"

	"dt-server/common/logger"
	infmysql "dt-server/internal/infra/mysql"
	infmq "dt-server/internal/infra/rocketmq"
	"dt-server/internal/model"

	"go.uber.org/zap"
)

// StartOutboxDispatcher 启动 Outbox 分发器，支持通过 ctx 优雅退出
// 仅当 MQ 已启用时运行。
func StartOutboxDispatcher(ctx context.Context, wg *sync.WaitGroup) {
	if !infmq.Enabled() {
		return
	}
	wg.Add(1)
	pub := infmq.PublisherInstance()
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer wg.Done()

		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c, cancel := context.WithTimeout(ctx, 2*time.Second)
				rows, err := model.ListOutboxPending(c, infmysql.SQLX(), 100)
				cancel()
				if err != nil {
					logger.Warn("outbox: list pending failed", zap.Error(err))
					continue
				}
				for _, r := range rows {
					// publish
					if err := pub.Publish(r.Topic, []byte(r.Payload)); err != nil {
						_ = model.MarkOutboxFailed(ctx, infmysql.SQLX(), r.ID, truncateErr(err))
						continue
					}
					if err := model.MarkOutboxSent(ctx, infmysql.SQLX(), r.ID); err != nil {
						logger.Warn("outbox: mark sent failed", zap.Int64("id", r.ID), zap.Error(err))
					}
				}
			}
		}
	}()
}

func truncateErr(err error) string {
	b, _ := json.Marshal(map[string]string{"error": err.Error()})
	if len(b) > 240 {
		return string(b[:240])
	}
	return string(b)
}

// StartInboxConsumer 启动 RocketMQ v5 SimpleConsumer，将消息可靠落库至 inbox 表（去重）
// 配置项：
// - rocketmq_endpoint 或 rocketmq_namesrv
// - rocketmq_consumer_group
// - rocketmq_consume_topics（可空，默认回退到 rocketmq_producer_topics）
func StartInboxConsumer(ctx context.Context, wg *sync.WaitGroup) {
	// Ensure RocketMQ SDK logs go to console instead of /logs
	rmq.ResetLogger()

	endpoint, _ := beego.AppConfig.String("rocketmq_endpoint")
	if endpoint == "" {
		endpoint, _ = beego.AppConfig.String("rocketmq_namesrv")
	}
	if endpoint == "" {
		return
	}
	// sanitize endpoint: trim, strip scheme, pick first if contains ',' or ';'
	endpoint = strings.TrimSpace(endpoint)
	endpoint = strings.TrimPrefix(strings.TrimPrefix(endpoint, "http://"), "https://")
	if idx := strings.IndexAny(endpoint, ",;"); idx > 0 {
		endpoint = strings.TrimSpace(endpoint[:idx])
	}
	logger.Info("[mq] consumer endpoint", zap.String("endpoint", endpoint))

	group, _ := beego.AppConfig.String("rocketmq_consumer_group")
	if group == "" {
		logger.Warn("[mq] consumer not started: empty rocketmq_consumer_group")
		return
	}
	topicsStr, _ := beego.AppConfig.String("rocketmq_consume_topics")
	if topicsStr == "" {
		topicsStr, _ = beego.AppConfig.String("rocketmq_producer_topics")
	}
	if topicsStr == "" {
		logger.Warn("[mq] consumer not started: empty topics")
		return
	}
	ak, _ := beego.AppConfig.String("rocketmq_access_key")
	sk, _ := beego.AppConfig.String("rocketmq_secret_key")
	if strings.TrimSpace(ak) == "" || strings.TrimSpace(sk) == "" {
		logger.Warn("[mq] consumer not started: missing access/secret key")
		return
	}
	cfg := &rmq.Config{Endpoint: endpoint, ConsumerGroup: group}
	cfg.Credentials = &credentials.SessionCredentials{AccessKey: ak, AccessSecret: sk}

	// 构造订阅表达式：多个 topic，默认 SUB_ALL
	subs := map[string]*rmq.FilterExpression{}
	for _, t := range strings.Split(topicsStr, ",") {
		t = strings.TrimSpace(strings.ReplaceAll(t, ".", "_"))
		if t == "" {
			continue
		}
		subs[t] = rmq.SUB_ALL
	}

	awaitDuration := 5 * time.Second
	maxMessageNum := int32(16)
	invisibleDuration := 20 * time.Second

	// 尝试启动 SimpleConsumer（带重试，避免容器刚启动未就绪导致一次性失败）
	var sc rmq.SimpleConsumer
	var err error
	for i := 0; i < 6; i++ { // 最长约 6*3s = 18s
		sc, err = rmq.NewSimpleConsumer(cfg,
			rmq.WithSimpleAwaitDuration(awaitDuration),
			rmq.WithSimpleSubscriptionExpressions(subs),
		)
		if err == nil {
			if e := sc.Start(); e == nil {
				break
			} else {
				err = e
			}
		}
		logger.Warn("[mq] simple consumer start retry", zap.Int("attempt", i+1), zap.Error(err))
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		logger.Error("[mq] start simple consumer failed", zap.Error(err))
		return
	}
	logger.Info("[mq] inbox consumer started", zap.String("group", group), zap.String("topics", topicsStr))

	wg.Add(1)

	go func() {
		defer wg.Done()

		defer sc.GracefulStop()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				mvs, err := sc.Receive(ctx, maxMessageNum, invisibleDuration)
				if err != nil {
					// 上下文取消则直接退出
					if ctx.Err() != nil {
						return
					}
					logger.Warn("[mq] receive error", zap.Error(err))
					continue
				}
				for _, mv := range mvs {
					id := mv.GetMessageId()
					topic := mv.GetTopic()
					body := mv.GetBody()
					if err := model.UpsertInbox(ctx, infmysql.SQLX(), id, topic, string(body), time.Now().UnixMilli()); err != nil {
						logger.Warn("[mq] upsert inbox failed", zap.String("id", id), zap.String("topic", topic), zap.Error(err))
						continue
					}
					var payload map[string]any
					if err := json.Unmarshal(body, &payload); err == nil {
						if evt, ok := payload["event"].(string); ok && evt == "game_drawn" {
							roundID, _ := payload["game_round_id"].(string)
							result, _ := payload["result"].(string)
							logger.Info("[mq] consumed draw result", zap.String("round_id", roundID), zap.String("result", result))
						}
					}
					if err := sc.Ack(ctx, mv); err != nil {
						logger.Warn("[mq] ack failed", zap.String("id", id), zap.Error(err))
					}
				}
			}
		}
	}()
}
