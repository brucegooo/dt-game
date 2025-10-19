-- ============================================================================
-- 龙虎游戏服务数据库初始化脚本
-- 版本：v1.0
-- 日期：2025-10-19
-- 描述：包含所有表结构、索引、注释和默认数据
-- ============================================================================

-- 创建数据库
CREATE DATABASE IF NOT EXISTS dt_game;
USE dt_game;

-- 设置字符集和时区
-- SET time_zone = '+00:00';
-- SET NAMES utf8mb4 COLLATE utf8mb4_0900_ai_ci;

-- ============================================================================
-- 1. 用户信息表 (customers)
-- 描述：存储用户基本信息和余额，支持多平台用户
-- ============================================================================
CREATE TABLE IF NOT EXISTS `customers` (
  `user_id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '用户ID（内部唯一标识）',
  `platform_id` TINYINT NOT NULL DEFAULT 0 COMMENT '平台ID: 0=系统默认, 1=平台A, 2=平台B, 99=演示平台',
  `platform_user_id` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '平台的用户ID（平台侧的用户标识）',
  `username` VARCHAR(50) NOT NULL COMMENT '用户名(全局唯一)',
  `balance` DECIMAL(18,2) UNSIGNED NOT NULL DEFAULT 0.00 COMMENT '当前可用余额',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态: 1=启用 2=禁用',
  `created_at` BIGINT UNSIGNED NOT NULL COMMENT '创建时间(13位毫秒时间戳)',
  `updated_at` BIGINT UNSIGNED NOT NULL COMMENT '更新时间(13位毫秒时间戳)',
  PRIMARY KEY (`user_id`),
  UNIQUE KEY `username` (`username`),
  UNIQUE KEY `uk_platform_user` (`platform_id`, `platform_user_id`),
  INDEX `idx_username` (`username`),
  INDEX `idx_status` (`status`),
  INDEX `idx_platform_id` (`platform_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='用户信息表';

-- ============================================================================
-- 2. 资金账本表 (wallet_ledger)
-- 描述：记录所有资金变动流水，用于审计和对账
-- ============================================================================
CREATE TABLE IF NOT EXISTS `wallet_ledger` (
  `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '账本流水ID',
  `user_id` BIGINT NOT NULL COMMENT '用户ID',
  `biz_type` TINYINT NOT NULL COMMENT '业务类型: 1=bet 下注 2=settle 结算 3=refund 退款 4=adjust 后台调整',
  `biz_type_str` VARCHAR(32) NOT NULL COMMENT '业务类型(字符串, 用于查询)',
  `amount` DECIMAL(18,2) UNSIGNED NOT NULL COMMENT '变动金额(非负数); 方向由 before_amount/after_amount 和 业务类型 biz_type推导',
  `before_amount` DECIMAL(18,2) UNSIGNED NOT NULL COMMENT '变动前余额快照(非负数)',
  `after_amount` DECIMAL(18,2) UNSIGNED NOT NULL COMMENT '变动后余额快照(非负数)',
  `currency` VARCHAR(8) NOT NULL DEFAULT 'CNY' COMMENT '币种',
  `bill_no` VARCHAR(64) NOT NULL COMMENT '业务单号/幂等号(与订单/结算对应)',
  `game_round_id` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '关联局ID(无则留空)',
  `game_id` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '游戏ID',
  `room_id` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '房间ID',
  `remark` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '备注(简要说明 最好写清楚业务类型和变动方向)',
  `trace_id` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '链路追踪ID',
  `created_at` BIGINT UNSIGNED NOT NULL COMMENT '创建时间(13位毫秒时间戳)',
  PRIMARY KEY (`id`),
  INDEX `idx_user_time` (`user_id`, `created_at`),
  INDEX `idx_bill` (`bill_no`),
  INDEX `idx_game_round` (`game_round_id`),
  INDEX `idx_trace` (`trace_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='资金流水账本';

-- ============================================================================
-- 3. 注单表 (orders)
-- 描述：记录所有投注订单，支持多平台用户
-- 注意：实际应用中数据量会很大，需要考虑分库分表
-- ============================================================================
CREATE TABLE IF NOT EXISTS `orders` (
  `bill_no` VARCHAR(64) NOT NULL COMMENT '注单号(唯一,防重)',
  `room_id` VARCHAR(32) NOT NULL COMMENT '房间ID',
  `game_round_id` VARCHAR(64) NOT NULL COMMENT '局ID',
  `game_id` VARCHAR(32) NOT NULL COMMENT '游戏ID',
  `user_id` BIGINT NOT NULL COMMENT '用户ID（内部用户ID）',
  `platform_id` TINYINT NOT NULL DEFAULT 0 COMMENT '平台ID: 0=系统默认, 1=平台A, 2=平台B, 99=演示平台',
  `platform_user_id` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '平台的用户ID',
  `user_name` VARCHAR(50) NOT NULL COMMENT '用户名快照',
  `bet_amount` DECIMAL(18,2) UNSIGNED NOT NULL COMMENT '投注金额(非负数)',
  `play_type` TINYINT NOT NULL COMMENT '下注类型: 1=龙 2=虎 3=和',
  `bet_status` TINYINT NOT NULL DEFAULT 1 COMMENT '下注状态: 1=创建 2=成功 3=失败',
  `bet_time` BIGINT UNSIGNED NOT NULL COMMENT '下注时间(13位毫秒时间戳)',
  `bill_status` TINYINT NOT NULL DEFAULT 1 COMMENT '订单状态: 1=待结算 2=已结算 3=已取消',
  `game_result` TINYINT NOT NULL DEFAULT 0 COMMENT '游戏结果: 0=未开奖 1=dragon 2=tiger 3=tie',
  `win_amount` DECIMAL(18,2) UNSIGNED NOT NULL DEFAULT 0.00 COMMENT '派彩金额(结算后写入, 非负数)',
  `bet_odds` DECIMAL(8,4) NOT NULL COMMENT '赔率(下单时快照)',
  `currency` VARCHAR(8) NOT NULL DEFAULT 'CNY' COMMENT '币种',
  `idempotency_key` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '请求幂等键(可选)',
  `trace_id` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '链路追踪ID',
  `created_at` BIGINT UNSIGNED NOT NULL COMMENT '创建时间(13位毫秒时间戳)',
  `updated_at` BIGINT UNSIGNED NOT NULL COMMENT '更新时间(13位毫秒时间戳)',
  PRIMARY KEY (`bill_no`),
  INDEX `idx_user` (`user_id`),
  INDEX `idx_round` (`game_round_id`),
  INDEX `idx_status` (`bill_status`),
  INDEX `idx_time` (`bet_time`),
  INDEX `idx_game_result` (`game_result`),
  INDEX `idx_platform_user` (`platform_id`, `platform_user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='注单表';

-- ============================================================================
-- 4. 局信息表 (game_round_info)
-- 描述：记录每一局游戏的完整信息和状态
-- ============================================================================
CREATE TABLE IF NOT EXISTS `game_round_info` (
  `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `game_round_id` VARCHAR(64) NOT NULL COMMENT '局ID(唯一)',
  `game_id` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '游戏ID',
  `room_id` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '房间ID',
  `bet_start_time` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '开始下注时间(13位毫秒时间戳; 未设置=0)',
  `bet_stop_time` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '停止下注时间(13位毫秒时间戳; 未设置=0)',
  `game_draw_time` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '开奖时间(13位毫秒时间戳; 未设置=0)',
  `card_list` VARCHAR(256) NOT NULL DEFAULT '' COMMENT '牌面列表(JSON字符串; 存储为字符串)',
  `game_result` TINYINT NOT NULL DEFAULT 0 COMMENT '本局结果: 未设置=0; 1=dragon 2=tiger 3=tie',
  `game_result_str` VARCHAR(20) NOT NULL DEFAULT '' COMMENT '本局结果(冗余字符串: dragon|tiger|tie)',
  `game_status` TINYINT NOT NULL DEFAULT 1 COMMENT '回合状态: 1=初始 2=下注中 3=封盘 4=已发牌 5=已开奖 6=已结束',
  `trace_id` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '链路追踪ID',
  `created_at` BIGINT UNSIGNED NOT NULL COMMENT '创建时间(13位毫秒时间戳)',
  `updated_at` BIGINT UNSIGNED NOT NULL COMMENT '更新时间(13位毫秒时间戳)',
  PRIMARY KEY (`id`),
  UNIQUE KEY `game_round_id` (`game_round_id`),
  INDEX `idx_game_round` (`game_round_id`),
  INDEX `idx_game_status` (`game_status`),
  INDEX `idx_times` (`bet_start_time`, `bet_stop_time`, `game_draw_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='局信息表';

-- ============================================================================
-- 5. 游戏事件审计表 (game_event_audit)
-- 描述：记录每一个游戏事件，用于审计和追溯
-- ============================================================================
CREATE TABLE IF NOT EXISTS `game_event_audit` (
  `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `game_round_id` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '局ID',
  `game_id` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '游戏ID',
  `room_id` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '房间ID',
  `event_type` TINYINT NOT NULL DEFAULT 0 COMMENT '事件类型: 1=game_start 2=game_stop 3=new_card 4=game_draw 5=game_end',
  `prev_state` VARCHAR(16) NOT NULL DEFAULT '' COMMENT '前状态(字符串，遵循 internal/state 定义)',
  `next_state` VARCHAR(16) NOT NULL DEFAULT '' COMMENT '后状态(字符串，遵循 internal/state 定义)',
  `payload` VARCHAR(1024) NOT NULL COMMENT '事件载荷(参数摘要; JSON字符串)',
  `operator` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '操作者(系统/管理员/来源IP)',
  `source` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '来源: http|mq|task 等',
  `trace_id` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '链路追踪ID',
  `created_at` BIGINT UNSIGNED NOT NULL COMMENT '创建时间(13位毫秒时间戳)',
  PRIMARY KEY (`id`),
  INDEX `idx_round_event` (`game_round_id`, `event_type`),
  INDEX `idx_time` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='游戏事件审计表';

-- ============================================================================
-- 6. Outbox 消息表 (outbox)
-- 描述：事务外发消息表，用于实现可靠消息发送
-- ============================================================================
CREATE TABLE IF NOT EXISTS `outbox` (
  `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `topic` VARCHAR(128) NOT NULL COMMENT '主题',
  `biz_key` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '业务主键(例如 bill_no 或 game_round_id)',
  `payload` VARCHAR(2048) NOT NULL COMMENT '消息内容(JSON字符串)',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态: 1=待发送 2=已发送 3=失败',
  `retry_count` INT NOT NULL DEFAULT 0 COMMENT '重试次数',
  `last_error` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '最后一次错误信息',
  `created_at` BIGINT UNSIGNED NOT NULL COMMENT '创建时间(13位毫秒时间戳)',
  `updated_at` BIGINT UNSIGNED NOT NULL COMMENT '更新时间(13位毫秒时间戳)',
  PRIMARY KEY (`id`),
  INDEX `idx_topic_status` (`topic`, `status`),
  INDEX `idx_biz_key` (`biz_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Outbox消息表';

-- ============================================================================
-- 7. Inbox 消息表 (inbox)
-- 描述：消息去重消费表，防止重复消费
-- ============================================================================
CREATE TABLE IF NOT EXISTS `inbox` (
  `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `message_id` VARCHAR(128) NOT NULL COMMENT '消息唯一ID',
  `topic` VARCHAR(128) NOT NULL COMMENT '主题',
  `payload` VARCHAR(2048) NOT NULL COMMENT '消息内容(JSON字符串)',
  `processed_at` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '处理完成时间(13位毫秒时间戳; 未处理=0)',
  `created_at` BIGINT UNSIGNED NOT NULL COMMENT '创建时间(13位毫秒时间戳)',
  PRIMARY KEY (`id`),
  UNIQUE KEY `message_id` (`message_id`),
  INDEX `idx_topic` (`topic`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='Inbox消息表';

-- ============================================================================
-- 8. 幂等键表 (idempotency_keys)
-- 描述：用于接口幂等性控制
-- ============================================================================
CREATE TABLE IF NOT EXISTS `idempotency_keys` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `idempotency_key` VARCHAR(128) NOT NULL COMMENT '幂等键',
  `purpose` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '用途(如 bet)',
  `ref` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '参考号(如 bill_no)',
  `created_at` BIGINT UNSIGNED NOT NULL COMMENT '创建时间(13位毫秒时间戳)',
  `is_delete` TINYINT(1) NOT NULL DEFAULT 1 COMMENT '是否删除：1正常，2删除',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_idempotency_key` (`idempotency_key`),
  INDEX `idx_purpose` (`purpose`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='幂等键表';

-- ============================================================================
-- 9. 平台配置表 (platforms)
-- 描述：存储接入平台的配置信息，用于多平台认证
-- ============================================================================
CREATE TABLE IF NOT EXISTS `platforms` (
  `platform_id` TINYINT NOT NULL COMMENT '平台ID',
  `platform_name` VARCHAR(50) NOT NULL COMMENT '平台名称',
  `app_key` VARCHAR(64) NOT NULL COMMENT '平台AppKey',
  `app_secret` VARCHAR(128) NOT NULL COMMENT '平台AppSecret',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态: 0=禁用, 1=启用',
  `ip_whitelist` TEXT COMMENT 'IP白名单（JSON数组）',
  `created_at` BIGINT UNSIGNED NOT NULL COMMENT '创建时间(13位毫秒时间戳)',
  `updated_at` BIGINT UNSIGNED NOT NULL COMMENT '更新时间(13位毫秒时间戳)',
  PRIMARY KEY (`platform_id`),
  UNIQUE KEY `app_key` (`app_key`),
  INDEX `idx_app_key` (`app_key`),
  INDEX `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci COMMENT='游戏平台配置表';

-- ============================================================================
-- 初始化数据：插入默认平台配置
-- ============================================================================
INSERT INTO `platforms` (`platform_id`, `platform_name`, `app_key`, `app_secret`, `status`, `created_at`, `updated_at`) VALUES
(0, '系统默认', 'system', 'system_secret_change_me', 1, UNIX_TIMESTAMP() * 1000, UNIX_TIMESTAMP() * 1000),
(99, '演示平台', 'demo_platform', 'demo_secret_123456', 1, UNIX_TIMESTAMP() * 1000, UNIX_TIMESTAMP() * 1000)
ON DUPLICATE KEY UPDATE
  `platform_name` = VALUES(`platform_name`),
  `updated_at` = UNIX_TIMESTAMP() * 1000;

  INSERT INTO dt_game.customers
(user_id, platform_id, platform_user_id, username, amount, status, created_at, updated_at)
VALUES(9999, 99, '9999', 'Chris', 1000000.00, 1, 1760284800000, 1760284800000);

-- ============================================================================
-- 脚本结束
-- ============================================================================