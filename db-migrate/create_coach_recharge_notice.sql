CREATE TABLE IF NOT EXISTS `coach_recharge_notice` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '充值待办ID',
  `coach_id` BIGINT UNSIGNED NOT NULL COMMENT '上报教练ID，仅作历史身份引用',
  `member_id` BIGINT UNSIGNED NOT NULL COMMENT '会员ID，仅作历史身份引用',
  `recharge_date` DATE NOT NULL DEFAULT (CURRENT_DATE) COMMENT '北京时间充值业务日期',
  `note` VARCHAR(500) NOT NULL COMMENT '教练上报备注',
  `status` VARCHAR(20) NOT NULL DEFAULT 'PENDING' COMMENT 'PENDING或ACKNOWLEDGED',
  `version` BIGINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '内容版本',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  `acknowledged_at` DATETIME NULL DEFAULT NULL COMMENT '管理员知悉时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_recharge_notice_coach_member_date` (`coach_id`,`member_id`,`recharge_date`),
  KEY `idx_recharge_notice_pending_order` (`status`,`recharge_date`,`updated_at`,`id`),
  KEY `idx_recharge_notice_member` (`member_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='教练上报的充值待办，不代表真实充值';
