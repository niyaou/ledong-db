#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
使用 Spark 进行 MySQL 数据库全量迁移
"""

import json
import pymysql
from pyspark.sql import SparkSession
import logging
import re
import os
import tempfile

logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)

# 根据ER图确定的表依赖关系
TABLE_DEPENDENCIES = {
    'court': [],
    'coach': [],
    'prepaid_card': [],
    'course': ['coach', 'court'],
    'charge': ['coach', 'prepaid_card'],
    'spend': ['course', 'prepaid_card'],
    'course_member': ['course', 'prepaid_card'],  # 修正拼写错误，member_id关联prepaid_card.id
}


def load_config(config_path='config.json'):
    """从配置文件加载数据库连接参数"""
    with open(config_path, 'r', encoding='utf-8') as f:
        config = json.load(f)
    source = config.get('source', {})
    target = config.get('target', {})
    # 支持全局驱动配置
    driver_jar = config.get('driver_jar', None)
    if driver_jar and 'driver_jar' not in source:
        source['driver_jar'] = driver_jar
    if driver_jar and 'driver_jar' not in target:
        target['driver_jar'] = driver_jar
    return source, target


def get_connection(db_config):
    """创建MySQL连接"""
    return pymysql.connect(
        host=db_config['host'],
        port=db_config['port'],
        user=db_config['user'],
        password=db_config['password'],
        database=db_config['database'],
        charset='utf8mb4'
    )


def get_existing_tables(conn):
    """获取数据库中所有存在的表（排除视图）"""
    with conn.cursor() as cursor:
        cursor.execute("""
            SELECT TABLE_NAME 
            FROM INFORMATION_SCHEMA.TABLES 
            WHERE TABLE_SCHEMA = DATABASE() 
            AND TABLE_TYPE = 'BASE TABLE'
        """)
        return [row[0] for row in cursor.fetchall()]


def get_table_dependencies(existing_tables):
    """分析表依赖关系，确定迁移顺序"""
    # 只处理存在的表
    filtered_deps = {t: deps for t, deps in TABLE_DEPENDENCIES.items() if t in existing_tables}
    # 过滤依赖，只保留存在的表
    for table, deps in filtered_deps.items():
        filtered_deps[table] = [d for d in deps if d in existing_tables]
    
    # 拓扑排序确定迁移顺序
    in_degree = {table: len(deps) for table, deps in filtered_deps.items()}
    queue = [table for table, degree in in_degree.items() if degree == 0]
    result = []
    
    while queue:
        table = queue.pop(0)
        result.append(table)
        for t, deps in filtered_deps.items():
            if table in deps:
                in_degree[t] -= 1
                if in_degree[t] == 0:
                    queue.append(t)
    
    return result


def extract_table_ddl(conn, table_name):
    """提取表结构定义（DDL）"""
    with conn.cursor() as cursor:
        cursor.execute(f"SHOW CREATE TABLE `{table_name}`")
        result = cursor.fetchone()
        return result[1] if result else None


def extract_indexes(conn, table_name):
    """提取索引定义"""
    with conn.cursor() as cursor:
        cursor.execute(f"SHOW INDEX FROM `{table_name}`")
        indexes = cursor.fetchall()
        
        index_map = {}
        for idx in indexes:
            idx_name = idx[2]
            if idx_name == 'PRIMARY':
                continue
            if idx_name not in index_map:
                index_map[idx_name] = {'columns': [], 'unique': idx[1] == 0, 'non_unique': idx[1] == 1}
            index_map[idx_name]['columns'].append(idx[4])
        
        return index_map


def extract_foreign_keys(conn, table_name):
    """提取外键约束"""
    with conn.cursor() as cursor:
        cursor.execute("""
            SELECT CONSTRAINT_NAME, COLUMN_NAME, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME
            FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE
            WHERE TABLE_SCHEMA = DATABASE()
            AND TABLE_NAME = %s
            AND REFERENCED_TABLE_NAME IS NOT NULL
        """, (table_name,))
        return cursor.fetchall()


def parse_additional_indexes(index_sql_path='db_index_crate.sql', target_database=None):
    """解析额外的索引SQL文件，返回索引定义字典
    
    Args:
        index_sql_path: 索引SQL文件路径
        target_database: 目标数据库名称（暂未使用，保留用于未来扩展）
    
    Returns:
        dict: {table_name: [(index_name, columns, is_unique), ...]}
    """
    indexes_dict = {}
    
    try:
        with open(index_sql_path, 'r', encoding='utf-8') as f:
            content = f.read()
        
        # 解析CREATE INDEX语句
        # 匹配模式：CREATE [UNIQUE] INDEX index_name ON `db`.table(column)
        # 例如: CREATE INDEX idx_charge_coach_id ON `ledong-membership`.charge(coach_id);
        pattern = r'CREATE\s+(?:UNIQUE\s+)?INDEX\s+`?(\w+)`?\s+ON\s+`?[^`]+`?\.`?(\w+)`?\s*\(([^)]+)\)'
        
        for line in content.split('\n'):
            line = line.strip()
            # 跳过注释和空行
            if not line or line.startswith('--'):
                continue
            
            # 匹配CREATE INDEX语句
            match = re.search(pattern, line, re.IGNORECASE)
            if match:
                index_name = match.group(1)
                table_name = match.group(2)
                columns_str = match.group(3)
                
                # 检查是否为UNIQUE索引
                is_unique = 'UNIQUE' in line.upper()
                
                # 解析列名，移除反引号
                columns = [col.strip().strip('`') for col in columns_str.split(',')]
                
                if table_name not in indexes_dict:
                    indexes_dict[table_name] = []
                indexes_dict[table_name].append((index_name, columns, is_unique))
        
        total_count = sum(len(v) for v in indexes_dict.values())
        if total_count > 0:
            logger.info(f"从 {index_sql_path} 解析到 {total_count} 个额外索引，涉及 {len(indexes_dict)} 个表")
        return indexes_dict
        
    except FileNotFoundError:
        logger.warning(f"索引SQL文件 {index_sql_path} 不存在，跳过额外索引创建")
        return {}
    except Exception as e:
        logger.warning(f"解析索引SQL文件失败: {e}")
        import traceback
        logger.debug(traceback.format_exc())
        return {}


def create_table_structure(target_conn, ddl, table_name=None):
    """在新库创建表结构"""
    # 移除AUTO_INCREMENT，避免冲突
    ddl_clean = re.sub(r'AUTO_INCREMENT=\d+', '', ddl)
    # 移除表选项中的某些特定设置
    ddl_clean = re.sub(r'ENGINE=\w+', 'ENGINE=InnoDB', ddl_clean)
    # 添加 IF NOT EXISTS，支持重复执行
    if 'IF NOT EXISTS' not in ddl_clean.upper():
        ddl_clean = re.sub(r'CREATE TABLE', 'CREATE TABLE IF NOT EXISTS', ddl_clean, flags=re.IGNORECASE)
    
    # 特殊处理：spend表的description字段从varchar改为float
    if table_name == 'spend':
        # 匹配description字段定义，将varchar类型改为float
        # 匹配模式：`description` varchar(...) 或 `description` VARCHAR(...)
        ddl_clean = re.sub(
            r"`description`\s+varchar\([^)]+\)",
            "`description` float",
            ddl_clean,
            flags=re.IGNORECASE
        )
        logger.info("已修改spend表的description字段类型为float")
    
    with target_conn.cursor() as cursor:
        try:
            cursor.execute(ddl_clean)
            target_conn.commit()
            return True
        except Exception as e:
            target_conn.rollback()
            logger.error(f"创建表结构失败: {e}")
            return False


def get_jdbc_url(db_config):
    """生成JDBC URL"""
    return f"jdbc:mysql://{db_config['host']}:{db_config['port']}/{db_config['database']}?useSSL=false&useUnicode=true&characterEncoding=utf8"


def get_jdbc_properties(db_config):
    """生成JDBC连接属性"""
    return {
        "user": db_config['user'],
        "password": db_config['password'],
        "driver": "com.mysql.cj.jdbc.Driver"
    }


def migrate_table_data(spark, source_config, target_config, table_name):
    """使用Spark迁移表数据"""
    from pyspark.sql.functions import col, when, isnan, isnull
    from pyspark.sql.types import FloatType
    
    source_url = get_jdbc_url(source_config)
    target_url = get_jdbc_url(target_config)
    source_props = get_jdbc_properties(source_config)
    target_props = get_jdbc_properties(target_config)
    
    logger.info(f"开始迁移表: {table_name}")
    
    # 读取源表数据
    try:
        df = spark.read.jdbc(
            url=source_url,
            table=table_name,
            properties=source_props
        )
    except Exception as e:
        if "ClassNotFoundException" in str(e) or "com.mysql.cj.jdbc.Driver" in str(e):
            logger.error("MySQL JDBC驱动未找到！")
            logger.error("解决方案：")
            logger.error("1. 下载MySQL JDBC驱动: https://dev.mysql.com/downloads/connector/j/")
            logger.error("2. 在config.json中添加driver_jar配置，指向下载的jar文件路径")
            logger.error("   例如: \"driver_jar\": \"mysql-connector-java-8.0.33.jar\"")
            logger.error("3. 或者使用spark-submit运行时添加 --packages mysql:mysql-connector-java:8.0.33")
        raise
    
    # 特殊处理：spend表的description字段从varchar转换为float
    if table_name == 'spend' and 'description' in df.columns:
        logger.info("转换spend表的description字段：varchar -> float")
        # 将description字段从字符串转换为float
        # 处理空值和无法转换的值（设为0）
        df = df.withColumn(
            'description',
            when(col('description').isNull() | (col('description') == ''), 0.0)
            .otherwise(col('description').cast(FloatType()))
        )
        # 处理转换失败的情况（NaN），设为0
        df = df.withColumn(
            'description',
            when(isnan(col('description')) | isnull(col('description')), 0.0)
            .otherwise(col('description'))
        )
    
    row_count = df.count()
    logger.info(f"表 {table_name} 共 {row_count} 条记录")
    
    if row_count == 0:
        logger.info(f"表 {table_name} 为空，跳过数据迁移")
        return True
    
    # 检查目标表是否已有数据
    try:
        target_df = spark.read.jdbc(url=target_url, table=table_name, properties=target_props)
        target_count = target_df.count()
        if target_count > 0:
            logger.info(f"表 {table_name} 目标表已有 {target_count} 条记录，跳过数据迁移（避免重复）")
            return True
    except Exception:
        pass  # 表不存在或读取失败，继续执行写入
    
    # 并行写入：每批500条，限制最大并发数避免超过MySQL连接数
    try:
        batch_size = 500
        max_concurrent = 50  # 最大并发连接数，避免超过MySQL限制
        num_partitions = min(max(row_count // batch_size, 1), max_concurrent)
        logger.info(f"表 {table_name} 将使用 {num_partitions} 个并发连接写入（每批约 {batch_size} 条）")
        df_repartitioned = df.repartition(num_partitions)
        
        df_repartitioned.write.jdbc(
            url=target_url,
            table=table_name,
            mode="append",
            properties=target_props
        )
        
        logger.info(f"表 {table_name} 迁移完成")
        return True
    except Exception as e:
        logger.error(f"表 {table_name} 迁移失败: {e}")
        return False


def create_indexes_and_constraints(source_conn, target_conn, table_name, additional_indexes=None):
    """创建索引和外键约束
    
    Args:
        source_conn: 源数据库连接
        target_conn: 目标数据库连接
        table_name: 表名
        additional_indexes: 额外索引字典 {table_name: [(index_name, columns, is_unique), ...]}
    """
    try:
        indexes = extract_indexes(source_conn, table_name)
    except Exception as e:
        # 如果源库中没有该表（如中间表），使用空字典
        logger.debug(f"源库中不存在表 {table_name}，跳过源索引提取: {e}")
        indexes = {}
    with target_conn.cursor() as cursor:
        # 先创建源表中的索引
        for idx_name, idx_info in indexes.items():
            unique = "UNIQUE" if idx_info['unique'] else ""
            columns = ",".join([f"`{col}`" for col in idx_info['columns']])
            try:
                cursor.execute(f"CREATE {unique} INDEX `{idx_name}` ON `{table_name}` ({columns})")
                logger.info(f"创建索引: {table_name}.{idx_name}")
            except Exception as e:
                if "1061" in str(e) or "Duplicate key" in str(e):
                    logger.debug(f"索引已存在，跳过: {table_name}.{idx_name}")
                else:
                    logger.warning(f"创建索引失败 {table_name}.{idx_name}: {e}")
        
        # 创建额外的索引（来自db_index_crate.sql）
        if additional_indexes and table_name in additional_indexes:
            for index_name, columns, is_unique in additional_indexes[table_name]:
                # 检查索引是否已存在（可能源表中已经有同名索引）
                if index_name in indexes:
                    logger.debug(f"索引 {table_name}.{index_name} 已从源表创建，跳过")
                    continue
                
                unique = "UNIQUE" if is_unique else ""
                columns_str = ",".join([f"`{col}`" for col in columns])
                try:
                    cursor.execute(f"CREATE {unique} INDEX `{index_name}` ON `{table_name}` ({columns_str})")
                    logger.info(f"创建额外索引: {table_name}.{index_name} 在字段 ({columns_str})")
                except Exception as e:
                    if "1061" in str(e) or "Duplicate key" in str(e):
                        logger.debug(f"额外索引已存在，跳过: {table_name}.{index_name}")
                    else:
                        logger.warning(f"创建额外索引失败 {table_name}.{index_name}: {e}")
        
        # 提取外键约束（如果源库中存在该表）
        try:
            fks = extract_foreign_keys(source_conn, table_name)
        except Exception as e:
            logger.debug(f"源库中不存在表 {table_name}，跳过外键提取: {e}")
            fks = []
        
        for fk in fks:
            constraint_name, column_name, ref_table, ref_column = fk
            try:
                cursor.execute(f"""
                    ALTER TABLE `{table_name}` 
                    ADD CONSTRAINT `{constraint_name}` 
                    FOREIGN KEY (`{column_name}`) 
                    REFERENCES `{ref_table}` (`{ref_column}`)
                """)
                logger.info(f"创建外键: {table_name}.{constraint_name}")
            except Exception as e:
                if "1826" in str(e) or "Duplicate foreign key" in str(e):
                    logger.debug(f"外键已存在，跳过: {table_name}.{constraint_name}")
                else:
                    logger.warning(f"创建外键失败 {table_name}.{constraint_name}: {e}")
        
        target_conn.commit()


def verify_migration(source_conn, target_conn, table_name):
    """验证迁移结果"""
    with source_conn.cursor() as src_cursor, target_conn.cursor() as tgt_cursor:
        src_cursor.execute(f"SELECT COUNT(*) FROM `{table_name}`")
        src_count = src_cursor.fetchone()[0]
        
        tgt_cursor.execute(f"SELECT COUNT(*) FROM `{table_name}`")
        tgt_count = tgt_cursor.fetchone()[0]
        
        if src_count == tgt_count:
            logger.info(f"表 {table_name} 验证通过: {src_count} 条记录")
            return True
        else:
            logger.error(f"表 {table_name} 验证失败: 源表 {src_count} 条，目标表 {tgt_count} 条")
            return False


def main():
    """主流程控制"""
    config_path = 'config.json'
    
    # 加载配置
    source_config, target_config = load_config(config_path)
    logger.info("配置文件加载成功")
    
    # 创建连接
    source_conn = get_connection(source_config)
    target_conn = get_connection(target_config)
    
    # Windows环境下处理Hadoop依赖
    if os.name == 'nt':  # Windows系统
        hadoop_home = os.path.join(tempfile.gettempdir(), 'hadoop')
        bin_dir = os.path.join(hadoop_home, 'bin')
        os.makedirs(bin_dir, exist_ok=True)
        
        # 创建空的winutils.exe占位符（Windows批处理文件，执行后立即退出）
        winutils_path = os.path.join(bin_dir, 'winutils.exe')
        if not os.path.exists(winutils_path):
            # 创建一个最小的可执行文件占位符（使用Python脚本模拟）
            # 实际上创建批处理文件作为占位符
            bat_path = os.path.join(bin_dir, 'winutils.bat')
            with open(bat_path, 'w') as f:
                f.write('@echo off\nexit /b 0\n')
            # 如果存在bat，复制为exe（但最好的方法是下载真正的winutils.exe）
            # 这里我们设置环境变量，让Spark能找到这个目录
            logger.info(f"Windows环境：创建Hadoop目录: {hadoop_home}")
            logger.warning("提示：如果仍然报错，请下载winutils.exe并放到: " + bin_dir)
        
        os.environ['HADOOP_HOME'] = hadoop_home
        os.environ['hadoop.home.dir'] = hadoop_home
    
    # 初始化Spark
    spark_builder = SparkSession.builder \
        .appName("MySQL Migration") \
        .config("spark.sql.execution.arrow.pyspark.enabled", "false") \
        .config("spark.sql.warehouse.dir", os.path.join(tempfile.gettempdir(), "spark-warehouse")) \
        .config("spark.hadoop.fs.defaultFS", "file:///") \
        .master("local[*]")
    
    # 如果配置文件中指定了驱动jar路径，则加载它
    driver_jar = source_config.get('driver_jar') or target_config.get('driver_jar')
    if driver_jar:
        spark_builder = spark_builder.config("spark.jars", driver_jar)
        logger.info(f"使用驱动jar: {driver_jar}")
    else:
        logger.warning("未指定MySQL驱动jar路径，如果出现驱动找不到的错误，请在config.json中添加driver_jar配置")
    
    spark = spark_builder.getOrCreate()
    
    try:
        # 获取存在的表
        existing_tables = get_existing_tables(source_conn)
        logger.info(f"发现 {len(existing_tables)} 个表: {', '.join(existing_tables)}")
        
        # 获取迁移顺序
        table_order = get_table_dependencies(existing_tables)
        logger.info(f"迁移顺序: {', '.join(table_order)}")
        
        # 阶段1和2：迁移表结构
        logger.info("=" * 50)
        logger.info("阶段1: 迁移表结构")
        logger.info("=" * 50)
        
        ddl_cache = {}
        for table_name in table_order:
            try:
                ddl = extract_table_ddl(source_conn, table_name)
                if ddl:
                    ddl_cache[table_name] = ddl
                    if create_table_structure(target_conn, ddl, table_name):
                        logger.info(f"表结构创建成功: {table_name}")
                    else:
                        logger.error(f"表结构创建失败: {table_name}")
                        return
                else:
                    logger.warning(f"无法获取表 {table_name} 的DDL")
            except Exception as e:
                logger.error(f"提取表 {table_name} 的DDL失败: {e}")
                return
        
        # 阶段2：迁移数据
        logger.info("=" * 50)
        logger.info("阶段2: 迁移表数据")
        logger.info("=" * 50)
        
        for table_name in table_order:
            if not migrate_table_data(spark, source_config, target_config, table_name):
                logger.error(f"表 {table_name} 数据迁移失败，终止迁移")
                return
        
        # 阶段3：创建索引和外键约束
        logger.info("=" * 50)
        logger.info("阶段3: 创建索引和外键约束")
        logger.info("=" * 50)
        
        # 解析额外索引文件
        index_sql_path = os.path.join(os.path.dirname(__file__), 'db_index_crate.sql')
        additional_indexes = parse_additional_indexes(index_sql_path)
        
        for table_name in table_order:
            create_indexes_and_constraints(source_conn, target_conn, table_name, additional_indexes)
        
        # 为不在table_order中的表创建额外索引（如中间表course_member等）
        # 这些表可能没有在依赖关系中，但在索引SQL文件中有定义
        if additional_indexes:
            existing_tables_set = set(table_order)
            for table_name in additional_indexes.keys():
                if table_name not in existing_tables_set:
                    # 检查表是否存在于目标库
                    with target_conn.cursor() as cursor:
                        cursor.execute(f"""
                            SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES 
                            WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = %s
                        """, (table_name,))
                        if cursor.fetchone()[0] > 0:
                            logger.info(f"为表 {table_name} 创建额外索引（该表不在迁移列表中）")
                            create_indexes_and_constraints(source_conn, target_conn, table_name, additional_indexes)
        
        # 阶段4：验证迁移结果
        logger.info("=" * 50)
        logger.info("阶段4: 验证迁移结果")
        logger.info("=" * 50)
        
        all_verified = True
        for table_name in table_order:
            if not verify_migration(source_conn, target_conn, table_name):
                all_verified = False
        
        if all_verified:
            logger.info("=" * 50)
            logger.info("迁移完成！所有表验证通过")
            logger.info("=" * 50)
        else:
            logger.warning("迁移完成，但部分表验证失败，请检查日志")
            
    finally:
        source_conn.close()
        target_conn.close()
        spark.stop()


if __name__ == "__main__":
    main()

