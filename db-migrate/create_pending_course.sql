CREATE TABLE IF NOT EXISTS `pending_course` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '待审课程ID',
  `coach_id` BIGINT UNSIGNED NOT NULL COMMENT '教练ID',
  `court_id` BIGINT UNSIGNED NOT NULL COMMENT '校区ID',
  `start_time` DATETIME NOT NULL COMMENT '课程开始时间',
  `end_time` DATETIME NOT NULL COMMENT '课程结束时间',
  `duration` FLOAT NOT NULL COMMENT '课程时长（小时）',
  `course_type` INT NOT NULL COMMENT '-2体验未成单,-1体验成单,0订场,1班课,2私教',
  `is_adult` INT NULL DEFAULT 1 COMMENT '沿用course.is_adult语义',
  `description` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '课程备注',
  `members_data` JSON NOT NULL COMMENT '会员消费数组，体验课为[]',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_pending_coach_start_end` (`coach_id`,`start_time`,`end_time`),
  KEY `idx_pending_coach_start` (`coach_id`,`start_time`),
  KEY `idx_pending_court_start` (`court_id`,`start_time`),
  KEY `idx_pending_start` (`start_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='教练填报待审课程消息表';
