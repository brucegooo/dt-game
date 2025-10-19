# RocketMQ 启用指南

## 📋 概述

本项目使用 **RocketMQ 5.3.0** 作为消息队列，用于异步处理游戏结算消息。

**主要功能**：
- ✅ **Outbox 模式**：将结算消息先写入数据库 `outbox` 表，再异步发送到 RocketMQ
- ✅ **Inbox 模式**：消费 RocketMQ 消息并写入 `inbox` 表，实现消息去重和可靠消费
- ✅ **自动重试**：发送失败的消息会自动重试
- ✅ **幂等保证**：通过 `inbox` 表的唯一键保证消息不重复消费

---

## 🚀 快速启动

### 方式 1：使用 Docker Compose（推荐）

#### 1. 启动 RocketMQ 服务

```bash
# 启动 RocketMQ NameServer 和 Broker
docker-compose up -d rocketmq-namesrv rocketmq-broker

# 查看启动状态
docker-compose ps

# 查看日志
docker-compose logs -f rocketmq-namesrv
docker-compose logs -f rocketmq-broker
```

#### 2. 验证 RocketMQ 是否启动成功

```bash
# 检查 NameServer 端口
nc -zv localhost 9876

# 检查 Broker 端口
nc -zv localhost 10911

# 或者查看容器状态
docker ps | grep rocketmq
```

**预期输出**：
```
CONTAINER ID   IMAGE                    STATUS         PORTS
abc123def456   apache/rocketmq:5.3.0   Up 2 minutes   0.0.0.0:9876->9876/tcp
def456ghi789   apache/rocketmq:5.3.0   Up 2 minutes   0.0.0.0:10911->10911/tcp
```

---

### 方式 2：本地安装 RocketMQ

如果你不想使用 Docker，可以下载 RocketMQ 二进制包：

#### 1. 下载 RocketMQ

```bash
# 下载 RocketMQ 5.3.0
wget https://archive.apache.org/dist/rocketmq/5.3.0/rocketmq-all-5.3.0-bin-release.zip

# 解压
unzip rocketmq-all-5.3.0-bin-release.zip
cd rocketmq-all-5.3.0-bin-release
```

#### 2. 启动 NameServer

```bash
# Linux/Mac
nohup sh bin/mqnamesrv &

# Windows
start bin\mqnamesrv.cmd
```

#### 3. 启动 Broker

```bash
# Linux/Mac
nohup sh bin/mqbroker -n localhost:9876 &

# Windows
start bin\mqbroker.cmd -n localhost:9876
```

#### 4. 验证启动

```bash
# 查看进程
jps

# 应该看到 NamesrvStartup 和 BrokerStartup
```

---

## ⚙️ 配置 RocketMQ

### 1. 本地开发环境（Windows/Mac）

编辑 `config/windows.json` 或 `config/dev.json`：

```json
{
  "rocketmq": {
    "name_server": "127.0.0.1:9876",
    "producer_group": "game-producer",
    "consumer_group": "game-consumer",
    "topic_settle": "dt_settle",
    "access_key": "rocketmq",
    "secret_key": "rocketmq123"
  }
}
```

**注意**：
- `access_key` 和 `secret_key` 在开发环境中是**占位符**，因为 Broker 配置了 `aclEnable=false`
- 但代码中会检查这些字段是否为空，所以**必须提供非空值**

---

### 2. Docker 环境

编辑 `config/docker.json`：

```json
{
  "rocketmq": {
    "name_server": "rocketmq-namesrv:9876",
    "producer_group": "game-producer",
    "consumer_group": "game-consumer",
    "topic_settle": "dt_settle",
    "access_key": "rocketmq",
    "secret_key": "rocketmq123"
  }
}
```

---

### 3. Nacos 配置中心

如果使用 Nacos，编辑配置：

```yaml
rocketmq:
  name_server: "127.0.0.1:9876"
  producer_group: "game-producer"
  consumer_group: "game-consumer"
  topic_settle: "dt_settle"
  access_key: "rocketmq"
  secret_key: "rocketmq123"
```

---

## 🔍 验证 RocketMQ 是否启用

### 1. 启动应用

```bash
# 编译
go build -o dt-server ./cmd/server

# 启动（使用本地配置）
./dt-server
```

### 2. 查看启动日志

**成功启用 RocketMQ 的日志**：

```
[INFO] rocketmq producer config endpoint=127.0.0.1:9876 topics=dt_settle ak=rocketmq
[INFO] rocketmq: topics configured topics=[dt_settle]
[INFO] rocketmq: creating producer opts_count=1
[INFO] rocketmq: producer created, starting...
[INFO] rocketmq enabled endpoint=127.0.0.1:9876
```

**未启用 RocketMQ 的日志**（配置为空时）：

```
[WARN] rocketmq disabled: missing access/secret key while endpoint present
```

或者没有任何 RocketMQ 相关日志（endpoint 为空时）。

---

### 3. 测试消息发送

执行完整游戏流程，查看 `outbox` 表：

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

## 📊 RocketMQ 管理

### 1. 查看 Topic 列表

```bash
# 进入 Broker 容器
docker exec -it dt-rocketmq-broker sh

# 查看 Topic
sh mqadmin topicList -n rocketmq-namesrv:9876
```

### 2. 查看 Topic 详情

```bash
sh mqadmin topicStatus -n rocketmq-namesrv:9876 -t dt_settle
```

### 3. 查看消费者组

```bash
sh mqadmin consumerProgress -n rocketmq-namesrv:9876 -g game-consumer
```

### 4. 清空 Topic 数据（开发环境）

```bash
sh mqadmin deleteTopic -n rocketmq-namesrv:9876 -c DefaultCluster -t dt_settle
```

---

## 🐛 故障排查

### 问题 1：RocketMQ 未启用

**症状**：
- 启动日志中没有 "rocketmq enabled" 消息
- 或者看到 "rocketmq disabled" 警告

**可能原因**：
1. `name_server` 配置为空
2. `access_key` 或 `secret_key` 为空
3. RocketMQ 服务未启动

**解决方案**：
1. 检查配置文件，确保 `name_server`、`access_key`、`secret_key` 都不为空
2. 检查 RocketMQ 服务是否启动：`docker ps | grep rocketmq`
3. 检查端口是否可访问：`nc -zv localhost 9876`

---

### 问题 2：连接 RocketMQ 失败

**症状**：
- 启动日志中看到 "producer init failed" 或 "producer start failed"

**可能原因**：
1. RocketMQ 服务未启动
2. 端口不可访问
3. 网络配置错误

**解决方案**：
1. 检查 RocketMQ 容器状态：`docker-compose logs rocketmq-namesrv`
2. 检查端口映射：`docker-compose ps`
3. 如果使用 Docker，确保 `name_server` 配置为 `rocketmq-namesrv:9876`
4. 如果本地运行，确保 `name_server` 配置为 `127.0.0.1:9876`

---

### 问题 3：消息发送失败

**症状**：
- `outbox` 表中的消息一直是 `status = 0`
- 日志中看到发送错误

**可能原因**：
1. Topic 不存在（如果 `autoCreateTopicEnable=false`）
2. Broker 不可访问
3. 权限问题（ACL 启用但凭证错误）

**解决方案**：
1. 检查 Broker 配置：`autoCreateTopicEnable=true`（已配置）
2. 手动创建 Topic：
   ```bash
   docker exec -it dt-rocketmq-broker sh
   sh mqadmin updateTopic -n rocketmq-namesrv:9876 -c DefaultCluster -t dt_settle
   ```
3. 检查 ACL 配置：确保 `aclEnable=false`（开发环境）

---

### 问题 4：消费者未启动

**症状**：
- `inbox` 表中没有消息
- 日志中没有消费者相关日志

**可能原因**：
1. 消费者未启动（代码中可能被注释）
2. 配置错误

**解决方案**：
1. 检查 `cmd/server/main.go` 中是否调用了 `worker.StartInboxConsumer`
2. 检查配置中的 `consumer_group` 和 `topic_settle` 是否正确

---

## 📚 相关代码

### Producer 初始化

- **文件**：`internal/infra/rocketmq/mq.go`
- **函数**：`initMQ()`
- **配置读取**：从 `beego.AppConfig` 读取配置

### Outbox Dispatcher

- **文件**：`internal/worker/outbox_dispatcher.go`
- **函数**：`StartOutboxDispatcher()`
- **功能**：定时扫描 `outbox` 表，发送消息到 RocketMQ

### Inbox Consumer

- **文件**：`internal/worker/outbox_dispatcher.go`
- **函数**：`StartInboxConsumer()`
- **功能**：消费 RocketMQ 消息，写入 `inbox` 表

---

## ✅ 启用检查清单

- [ ] RocketMQ NameServer 已启动（端口 9876）
- [ ] RocketMQ Broker 已启动（端口 10911）
- [ ] 配置文件中 `name_server` 已填写
- [ ] 配置文件中 `access_key` 和 `secret_key` 已填写（非空即可）
- [ ] 配置文件中 `topic_settle` 已填写（如 `dt_settle`）
- [ ] 应用启动日志中看到 "rocketmq enabled"
- [ ] 执行游戏流程后，`outbox` 表中的消息 `status` 变为 1
- [ ] （可选）`inbox` 表中有消费记录

---

## 🎯 生产环境注意事项

### 1. 启用 ACL

生产环境应该启用 ACL 认证：

**修改 `docker/broker.conf`**：
```properties
aclEnable=true
```

**创建 ACL 配置文件** `docker/plain_acl.yml`：
```yaml
accounts:
  - accessKey: game_producer
    secretKey: your_strong_password_here
    whiteRemoteAddress:
    admin: false
    defaultTopicPerm: PUB
    defaultGroupPerm: PUB
    topicPerms:
      - dt_settle=PUB|SUB
    groupPerms:
      - game-producer=PUB
      - game-consumer=SUB
```

**更新配置文件**：
```json
{
  "rocketmq": {
    "access_key": "game_producer",
    "secret_key": "your_strong_password_here"
  }
}
```

### 2. 调整资源配置

生产环境应该增加内存和存储：

```yaml
environment:
  - JAVA_OPT_EXT=-Xms2g -Xmx2g
```

### 3. 持久化存储

确保使用持久化卷：

```yaml
volumes:
  - /data/rocketmq/logs:/home/rocketmq/logs
  - /data/rocketmq/store:/home/rocketmq/store
```

---

**启用完成时间**：_____________  
**启用人员**：_____________  
**验证人员**：_____________

