-- ============================================
-- 为所有表添加 deleted_at 列（支持 GORM 软删除）
-- 注意：数据库名称请根据实际情况修改
-- 如果列已存在，执行时会报错，可以在应用层忽略重复创建的错误
-- ============================================

-- charge 表
ALTER TABLE `workship`.charge 
ADD COLUMN `deleted_at` DATETIME(3) NULL DEFAULT NULL;
CREATE INDEX idx_charge_deleted_at ON `workship`.charge(deleted_at);

-- coach 表
ALTER TABLE `workship`.coach 
ADD COLUMN `deleted_at` DATETIME(3) NULL DEFAULT NULL;
CREATE INDEX idx_coach_deleted_at ON `workship`.coach(deleted_at);

-- course 表
ALTER TABLE `workship`.course 
ADD COLUMN `deleted_at` DATETIME(3) NULL DEFAULT NULL;
CREATE INDEX idx_course_deleted_at ON `workship`.course(deleted_at);

-- court 表
ALTER TABLE `workship`.court 
ADD COLUMN `deleted_at` DATETIME(3) NULL DEFAULT NULL;
CREATE INDEX idx_court_deleted_at ON `workship`.court(deleted_at);

-- prepaid_card 表
ALTER TABLE `workship`.prepaid_card 
ADD COLUMN `deleted_at` DATETIME(3) NULL DEFAULT NULL;
CREATE INDEX idx_prepaid_card_deleted_at ON `workship`.prepaid_card(deleted_at);


-- spend 表
ALTER TABLE `workship`.spend 
ADD COLUMN `deleted_at` DATETIME(3) NULL DEFAULT NULL;
CREATE INDEX idx_spend_deleted_at ON `workship`.spend(deleted_at);
