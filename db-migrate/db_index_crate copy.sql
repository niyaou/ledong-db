-- ============================================
-- 必须创建的索引（外键和核心查询字段）
-- 注意：数据库名称请根据实际情况修改
-- 如果索引已存在，执行时会报错，可以在应用层忽略重复创建的错误
-- ============================================

-- charge 表
CREATE INDEX idx_charge_coach_id ON `workship`.charge(coach_id);
CREATE INDEX idx_charge_prepaid_card_id ON `workship`.charge(prepaid_card_id);
CREATE INDEX idx_charge_charged_time ON `workship`.charge(charged_time);

-- spend 表
CREATE INDEX idx_spend_course_id ON `workship`.spend(course_id);
CREATE INDEX idx_spend_prepaid_card_id ON `workship`.spend(prepaid_card_id);

-- course 表
CREATE INDEX idx_course_coach_id ON `workship`.course(coach_id);
CREATE INDEX idx_course_court_id ON `workship`.course(court_id);
CREATE INDEX idx_course_start_time ON `workship`.course(start_time);

-- course_member 表
CREATE INDEX idx_course_member_course_id ON `workship`.course_member(course_id);
CREATE INDEX idx_course_member_member_id ON `workship`.course_member(member_id);

-- prepaid_card 表（与Go模型中的index标签对应）
CREATE INDEX idx_prepaid_card_number ON `workship`.prepaid_card(number);


-- coach 表（与Go模型中的index标签对应）
CREATE INDEX idx_coach_number ON `workship`.coach(number);

-- ============================================
-- 推荐创建的索引（唯一约束和常用查询）
-- ============================================

-- 唯一约束（根据业务需求决定，如果需要唯一性，请取消注释）
-- CREATE UNIQUE INDEX unique_prepaid_card_number ON `workship`.prepaid_card(number);
-- CREATE UNIQUE INDEX unique_coach_number ON `workship`.coach(number);
