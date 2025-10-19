-- ============================================
-- 添加结算幂等性保护
-- 创建时间: 2025-10-20
-- 说明: 防止重复结算漏洞
-- ============================================

-- 1. 在 game_round_info 表中添加 is_settled 字段
ALTER TABLE game_round_info 
ADD COLUMN is_settled TINYINT NOT NULL DEFAULT 0 COMMENT '是否已结算: 0=未结算 1=已结算' 
AFTER game_status;

-- 2. 添加索引以提高查询性能
ALTER TABLE game_round_info 
ADD INDEX idx_is_settled (is_settled);

-- 3. 创建结算日志表（用于审计和追溯）
CREATE TABLE IF NOT EXISTS settlement_log (
    id BIGINT AUTO_INCREMENT PRIMARY KEY COMMENT '自增ID',
    game_round_id VARCHAR(64) NOT NULL COMMENT '游戏回合ID',
    card_list VARCHAR(256) NOT NULL COMMENT '牌面信息',
    result VARCHAR(16) NOT NULL COMMENT '游戏结果: dragon|tiger|tie',
    total_orders INT NOT NULL DEFAULT 0 COMMENT '结算订单总数',
    total_payout DECIMAL(18,2) NOT NULL DEFAULT 0.00 COMMENT '总派彩金额',
    operator VARCHAR(64) NOT NULL DEFAULT 'system' COMMENT '操作人',
    trace_id VARCHAR(64) NOT NULL DEFAULT '' COMMENT '链路追踪ID',
    created_at BIGINT UNSIGNED NOT NULL COMMENT '创建时间（13位毫秒时间戳）',
    UNIQUE KEY uk_round (game_round_id) COMMENT '防止重复结算',
    KEY idx_created_at (created_at) COMMENT '时间索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='结算日志表（防止重复结算）';

-- 4. 添加新的游戏状态：settled(6)
-- 注意：这个状态在代码中定义，数据库中只需要支持存储即可
-- game_status: 1=created 2=betting 3=stopped 4=dealt 5=drawn 6=settled 7=ended

-- ============================================
-- 回滚脚本（如需回滚，请执行以下语句）
-- ============================================
-- ALTER TABLE game_round_info DROP COLUMN is_settled;
-- DROP TABLE IF EXISTS settlement_log;

