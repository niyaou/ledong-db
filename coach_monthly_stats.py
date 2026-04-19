#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
查询指定教练在2026年每个月的私教课、班课统计信息

用法:
    python coach_monthly_stats.py <coach_id>
    
示例:
    python coach_monthly_stats.py 42
"""

import sys
import io
import yaml
import pymysql
from wcwidth import wcswidth

# 修复 Windows 终端中文显示
sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8')
sys.stderr = io.TextIOWrapper(sys.stderr.buffer, encoding='utf-8')


def load_db_config(config_path='config.yaml'):
    """从 config.yaml 加载数据库配置"""
    with open(config_path, 'r', encoding='utf-8') as f:
        config = yaml.safe_load(f)
    db = config['database']
    return {
        'host': db['host'],
        'port': int(db['port']),
        'user': db['user'],
        'password': db['password'],
        'database': db['database'],
        'charset': 'utf8mb4'
    }


def get_coach_info(conn, coach_id):
    """获取教练基本信息"""
    with conn.cursor() as cursor:
        cursor.execute(
            "SELECT coach_id, name, number, level FROM coach WHERE coach_id = %s",
            (coach_id,)
        )
        return cursor.fetchone()


def query_monthly_stats(conn, coach_id):
    """查询教练2026年每月课程统计（含私教/班课人次和消课数）"""
    sql = """
    SELECT 
        c.c_month AS month,
        c.private_count,
        c.private_duration,
        c.group_count,
        c.group_duration,
        COALESCE(ps.private_attendance, 0) AS private_attendance,
        COALESCE(ps.private_consumption, 0) AS private_consumption,
        COALESCE(gs.group_attendance, 0) AS group_attendance,
        COALESCE(gs.group_consumption, 0) AS group_consumption
    FROM (
        SELECT 
            MONTH(start_time) AS c_month,
            SUM(CASE WHEN course_type = 2 THEN 1 ELSE 0 END) AS private_count,
            SUM(CASE WHEN course_type = 2 THEN duration ELSE 0 END) AS private_duration,
            SUM(CASE WHEN course_type = 1 THEN 1 ELSE 0 END) AS group_count,
            SUM(CASE WHEN course_type = 1 THEN duration ELSE 0 END) AS group_duration
        FROM course
        WHERE coach_id = %s
          AND YEAR(start_time) = 2026
          AND deleted_at IS NULL
        GROUP BY MONTH(start_time)
    ) c
    LEFT JOIN (
        SELECT 
            MONTH(c.start_time) AS s_month,
            COUNT(s.id) AS private_attendance,
            SUM(
                COALESCE(s.times, 0) 
                + COALESCE(s.annual_times, 0) 
                + CEILING(COALESCE(s.charge, 0) / 200)
            ) AS private_consumption
        FROM course c
        INNER JOIN spend s ON c.id = s.course_id AND s.deleted_at IS NULL
        WHERE c.coach_id = %s
          AND YEAR(c.start_time) = 2026
          AND c.course_type = 2
          AND c.deleted_at IS NULL
        GROUP BY MONTH(c.start_time)
    ) ps ON c.c_month = ps.s_month
    LEFT JOIN (
        SELECT 
            MONTH(c.start_time) AS s_month,
            COUNT(s.id) AS group_attendance,
            SUM(
                COALESCE(s.times, 0) 
                + COALESCE(s.annual_times, 0) 
                + CEILING(COALESCE(s.charge, 0) / 200)
            ) AS group_consumption
        FROM course c
        INNER JOIN spend s ON c.id = s.course_id AND s.deleted_at IS NULL
        WHERE c.coach_id = %s
          AND YEAR(c.start_time) = 2026
          AND c.course_type = 1
          AND c.deleted_at IS NULL
        GROUP BY MONTH(c.start_time)
    ) gs ON c.c_month = gs.s_month
    ORDER BY c.c_month
    """
    with conn.cursor() as cursor:
        cursor.execute(sql, (coach_id, coach_id, coach_id))
        return cursor.fetchall()


def ensure_float(value):
    """确保返回浮点数"""
    if value is None:
        return 0.0
    return float(value)


def format_percent(numerator, denominator):
    """格式化百分比"""
    if denominator == 0 or denominator is None:
        return "0.0%"
    return f"{numerator / denominator * 100:.1f}%"


# ---------- 手动表格渲染 ----------

def display_width(text):
    """计算文本的显示宽度（中文字符计为2）"""
    return wcswidth(str(text))


def pad_center(text, width):
    """居中对齐，按显示宽度填充空格"""
    text = str(text)
    w = display_width(text)
    if w >= width:
        return text
    left = (width - w) // 2
    right = width - w - left
    return " " * left + text + " " * right


def build_table(headers, rows, aligns=None):
    """
    手动构建 ASCII 表格
    headers: 表头列表
    rows: 数据行列表（每个元素是字符串列表）
    aligns: 每列对齐方式 ('left'|'center'|'right')，默认居中
    """
    if aligns is None:
        aligns = ["center"] * len(headers)

    # 计算每列最大显示宽度，并设置最小宽度避免终端字体差异导致错位
    MIN_COL_WIDTH = 12
    col_widths = []
    for i, h in enumerate(headers):
        max_w = display_width(h)
        for row in rows:
            if i < len(row):
                max_w = max(max_w, display_width(row[i]))
        # 取偶数宽度，且至少为 MIN_COL_WIDTH，给中文字符留出足够空间
        if max_w < MIN_COL_WIDTH:
            max_w = MIN_COL_WIDTH
        elif max_w % 2 == 1:
            max_w += 1
        col_widths.append(max_w)

    # 边框字符
    HORIZONTAL = "─"
    VERTICAL = "│"
    CROSS = "┼"
    TOP_LEFT = "┌"
    TOP_RIGHT = "┐"
    TOP_CROSS = "┬"
    BOTTOM_LEFT = "└"
    BOTTOM_RIGHT = "┘"
    BOTTOM_CROSS = "┴"
    LEFT_CROSS = "├"
    RIGHT_CROSS = "┤"
    MIDDLE_CROSS = "┼"

    def make_line(left, cross, right):
        parts = [HORIZONTAL * (w + 2) for w in col_widths]
        return left + cross.join(parts) + right

    def format_row(cells):
        parts = []
        for i, cell in enumerate(cells):
            w = col_widths[i]
            align = aligns[i] if i < len(aligns) else "center"
            if align == "right":
                text = str(cell)
                pad = w - display_width(text)
                parts.append(" " + (" " * pad + text if pad > 0 else text) + " ")
            elif align == "left":
                text = str(cell)
                pad = w - display_width(text)
                parts.append(" " + (text + " " * pad if pad > 0 else text) + " ")
            else:  # center
                parts.append(" " + pad_center(cell, w) + " ")
        return VERTICAL + VERTICAL.join(parts) + VERTICAL

    lines = []
    # 上边框
    lines.append(make_line(TOP_LEFT, TOP_CROSS, TOP_RIGHT))
    # 表头
    lines.append(format_row(headers))
    # 表头下边框（加粗分隔）
    lines.append(make_line(LEFT_CROSS, MIDDLE_CROSS, RIGHT_CROSS).replace("─", "═").replace("┼", "╪").replace("├", "╞").replace("┤", "╡").replace("┬", "╤").replace("┴", "╧"))
    # 数据行
    for row in rows:
        lines.append(format_row(row))
    # 下边框
    lines.append(make_line(BOTTOM_LEFT, BOTTOM_CROSS, BOTTOM_RIGHT))

    return "\n".join(lines)


# ----------------------------------


def main():
    if len(sys.argv) < 2:
        print("错误: 请提供 coach_id 参数")
        print("用法: python coach_monthly_stats.py <coach_id>")
        sys.exit(1)

    try:
        coach_id = int(sys.argv[1])
    except ValueError:
        print("错误: coach_id 必须是整数")
        sys.exit(1)

    # 加载配置并连接数据库
    try:
        db_config = load_db_config()
    except Exception as e:
        print(f"加载配置文件失败: {e}")
        sys.exit(1)

    conn = pymysql.connect(**db_config)
    try:
        # 获取教练信息
        coach = get_coach_info(conn, coach_id)
        if not coach:
            print(f"错误: 未找到教练 ID = {coach_id}")
            sys.exit(1)

        coach_id_db, coach_name, coach_number, coach_level = coach
        print(f"\n教练: {coach_name} (ID: {coach_id_db}, 手机号: {coach_number}, 等级: {coach_level})")
        print(f"统计年份: 2026年\n")

        # 查询统计数据
        rows = query_monthly_stats(conn, coach_id)

        if not rows:
            print("该教练在2026年暂无课程记录。\n")
            return

        headers = [
            "月份", "私教数量", "私教课时", "私教人次", "私教消课",
            "班课数量", "班课时", "班课人次", "班课消课",
            "总数量", "总课时", "私教数量占比", "私教课时占比"
        ]

        # 用于计算年度汇总
        total_private_count = 0
        total_private_duration = 0.0
        total_private_attendance = 0
        total_private_consumption = 0.0
        total_group_count = 0
        total_group_duration = 0.0
        total_group_attendance = 0
        total_group_consumption = 0.0

        table_rows = []

        for row in rows:
            (month, private_count, private_duration, group_count, group_duration,
             private_attendance, private_consumption,
             group_attendance, group_consumption) = row
            private_count = int(private_count or 0)
            private_duration = private_duration or 0.0
            private_attendance = int(private_attendance or 0)
            private_consumption = private_consumption or 0.0
            group_count = int(group_count or 0)
            group_duration = group_duration or 0.0
            group_attendance = int(group_attendance or 0)
            group_consumption = group_consumption or 0.0

            total_count = private_count + group_count
            total_dur = private_duration + group_duration

            total_private_count += private_count
            total_private_duration += private_duration
            total_private_attendance += private_attendance
            total_private_consumption += private_consumption
            total_group_count += group_count
            total_group_duration += group_duration
            total_group_attendance += group_attendance
            total_group_consumption += group_consumption

            table_rows.append([
                f"{month}月",
                str(private_count),
                f"{private_duration:.1f}",
                str(private_attendance),
                f"{private_consumption:.1f}",
                str(group_count),
                f"{group_duration:.1f}",
                str(group_attendance),
                f"{group_consumption:.1f}",
                str(total_count),
                f"{total_dur:.1f}",
                format_percent(private_count, total_count),
                format_percent(private_duration, total_dur),
            ])

        # 添加汇总行
        grand_total_count = total_private_count + total_group_count
        grand_total_duration = total_private_duration + total_group_duration

        table_rows.append([
            "全年汇总",
            str(total_private_count),
            f"{total_private_duration:.1f}",
            str(total_private_attendance),
            f"{total_private_consumption:.1f}",
            str(total_group_count),
            f"{total_group_duration:.1f}",
            str(total_group_attendance),
            f"{total_group_consumption:.1f}",
            str(grand_total_count),
            f"{grand_total_duration:.1f}",
            format_percent(total_private_count, grand_total_count),
            format_percent(total_private_duration, grand_total_duration),
        ])

        aligns = ["center"] * len(headers)
        print(build_table(headers, table_rows, aligns))
        print()

    except pymysql.MySQLError as e:
        print(f"数据库查询失败: {e}")
        sys.exit(1)
    finally:
        conn.close()


if __name__ == "__main__":
    main()
