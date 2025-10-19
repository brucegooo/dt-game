# RocketMQ 快速启动指南

## 🎯 一键启动

### Linux/Mac

```bash
# 启动 RocketMQ 和应用
./start-with-rocketmq.sh
```

### Windows

```powershell
# 启动 RocketMQ 和应用
.\start-with-rocketmq.ps1
```

---

## 📝 配置说明

所有配置文件已经更新，RocketMQ 默认配置如下：

| 配置项 | 值 | 说明 |
|--------|-----|------|
| `name_server` | `127.0.0.1:9876` | NameServer 地址（本地）<br>`rocketmq-namesrv:9876`（Docker） |
| `producer_group` | `game-producer` | 生产者组 |
| `consumer_group` | `game-consumer` | 消费者组 |
| `topic_settle` | `dt_settle` | 结算消息 Topic |
| `access_key` | `rocketmq` | 访问密钥（开发环境占位符） |
| `secret_key` | `rocketmq123` | 密钥（开发环境占位符） |

**注意**：开发环境中 ACL 已禁用（`aclEnable=false`），`access_key` 和 `secret_key` 只是占位符，但**必须非空**。

---

## ✅ 验证 RocketMQ 是否启用

### 1. 查看启动日志

**成功启用的日志**：

```
[INFO] rocketmq producer config endpoint=127.0.0.1:9876 topics=dt_settle ak=rocketmq
[INFO] rocketmq: topics configured topics=[dt_settle]
[INFO] rocketmq: creating producer opts_count=1
[INFO] rocketmq: producer created, starting...
[INFO] rocketmq enabled endpoint=127.0.0.1:9876
```

**未启用的日志**：

```
[WARN] rocketmq disabled: missing access/secret key while endpoint present
```

或者没有任何 RocketMQ 相关日志。

---

### 2. 测试消息发送

执行完整游戏流程：

1. 访问调试页面：`http://localhost:8087/debug`
2. 执行游戏流程：
   - 游戏开始 (event_type=1)
   - 用户投注
   - 游戏封盘 (event_type=2)
   - 发牌 (event_type=3)
   - 准备开奖 (event_type=4)
   - **开奖结算** ← 这一步会发送消息到 RocketMQ
   - 游戏结束 (event_type=5)

3. 查询 `outbox` 表：

```sql
-- 查看待发送的消息
SELECT * FROM outbox WHERE status = 0 ORDER BY created_at DESC LIMIT 10;

-- 查看已发送的消息
SELECT * FROM outbox WHERE status = 1 ORDER BY created_at DESC LIMIT 10;
```

**如果 RocketMQ 启用成功**：
- `outbox` 表中的消息 `status` 会从 0（待发送）变为 1（已发送）
- `sent_at` 字段会有时间戳

**如果 RocketMQ 未启用**：
- `outbox` 表中的消息会一直保持 `status = 0`
- 日志中会看到：`[mq disabled] drop message topic=dt_settle`

---

## 🔧 手动启动 RocketMQ

如果不想使用一键启动脚本，可以手动启动：

### 1. 启动 RocketMQ 服务

```bash
# 启动 NameServer 和 Broker
docker-compose up -d rocketmq-namesrv rocketmq-broker

# 查看状态
docker-compose ps

# 查看日志
docker-compose logs -f rocketmq-namesrv
docker-compose logs -f rocketmq-broker
```

### 2. 验证端口

```bash
# 检查 NameServer 端口
nc -zv localhost 9876

# 检查 Broker 端口
nc -zv localhost 10911
```

### 3. 启动应用

```bash
# 编译
go build -o dt-server ./cmd/server

# 启动
./dt-server
```

---

## 🛑 停止 RocketMQ

```bash
# 停止 RocketMQ 服务
docker-compose stop rocketmq-namesrv rocketmq-broker

# 或者完全删除容器
docker-compose down rocketmq-namesrv rocketmq-broker
```

---

## 📊 RocketMQ 管理命令

### 查看 Topic 列表

```bash
docker exec -it dt-rocketmq-broker sh
sh mqadmin topicList -n rocketmq-namesrv:9876
```

### 查看 Topic 详情

```bash
sh mqadmin topicStatus -n rocketmq-namesrv:9876 -t dt_settle
```

### 查看消费者组

```bash
sh mqadmin consumerProgress -n rocketmq-namesrv:9876 -g game-consumer
```

### 手动创建 Topic（如果自动创建失败）

```bash
sh mqadmin updateTopic -n rocketmq-namesrv:9876 -c DefaultCluster -t dt_settle
```

---

## 🐛 常见问题

### 问题 1：启动日志中没有 "rocketmq enabled"

**原因**：配置文件中的 `name_server`、`access_key` 或 `secret_key` 为空。

**解决方案**：
1. 检查配置文件（`config/windows.json` 或 `config/dev.json`）
2. 确保所有 RocketMQ 配置项都不为空
3. 重启应用

---

### 问题 2：端口不可访问

**原因**：RocketMQ 服务未启动或启动失败。

**解决方案**：
1. 检查容器状态：`docker-compose ps`
2. 查看日志：`docker-compose logs rocketmq-namesrv rocketmq-broker`
3. 重启服务：`docker-compose restart rocketmq-namesrv rocketmq-broker`

---

### 问题 3：消息一直是待发送状态

**原因**：RocketMQ 未启用或连接失败。

**解决方案**：
1. 检查启动日志，确认 "rocketmq enabled"
2. 检查端口是否可访问：`nc -zv localhost 9876`
3. 查看应用日志，查找错误信息

---

## 📚 相关文档

- **[ROCKETMQ_SETUP.md](ROCKETMQ_SETUP.md)** - 详细的 RocketMQ 配置和管理指南
- **[README.md](README.md)** - 项目总体文档
- **[DEPLOYMENT_GUIDE.md](DEPLOYMENT_GUIDE.md)** - 部署指南

---

## 🎉 总结

✅ **配置文件已更新**：所有配置文件中的 RocketMQ 配置已填写  
✅ **启动脚本已创建**：`start-with-rocketmq.sh` 和 `start-with-rocketmq.ps1`  
✅ **Broker 配置已优化**：ACL 已禁用，自动创建 Topic 已启用  
✅ **文档已完善**：详细的配置和故障排查指南  

现在你可以：
1. 运行 `./start-with-rocketmq.sh`（Linux/Mac）或 `.\start-with-rocketmq.ps1`（Windows）
2. 访问调试页面：`http://localhost:8087/debug`
3. 执行完整游戏流程，验证消息发送

RocketMQ 已经准备就绪！🚀

