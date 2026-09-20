-- 为“单次班课”（course_type=3）保存独立上报人数。
-- 迁移可重复执行；旧课程和其他课程类型统一使用默认值 0。

SET @pending_participant_count_sql = IF(
  EXISTS(
    SELECT 1
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'pending_course'
      AND COLUMN_NAME = 'participant_count'
  ),
  'SELECT 1',
  'ALTER TABLE `pending_course` ADD COLUMN `participant_count` INT NOT NULL DEFAULT 0 COMMENT ''单次班课上报人数，其他类型为0'' AFTER `course_type`'
);
PREPARE pending_participant_count_stmt FROM @pending_participant_count_sql;
EXECUTE pending_participant_count_stmt;
DEALLOCATE PREPARE pending_participant_count_stmt;

SET @course_participant_count_sql = IF(
  EXISTS(
    SELECT 1
    FROM information_schema.COLUMNS
    WHERE TABLE_SCHEMA = DATABASE()
      AND TABLE_NAME = 'course'
      AND COLUMN_NAME = 'participant_count'
  ),
  'SELECT 1',
  'ALTER TABLE `course` ADD COLUMN `participant_count` INT NOT NULL DEFAULT 0 COMMENT ''单次班课上报人数，其他类型为0'' AFTER `course_type`'
);
PREPARE course_participant_count_stmt FROM @course_participant_count_sql;
EXECUTE course_participant_count_stmt;
DEALLOCATE PREPARE course_participant_count_stmt;
