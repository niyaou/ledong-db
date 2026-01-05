#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
将指定用户的充值记录和课程记录的校区从"雅居乐"改为"领馆"
"""

import json
import sys
import pymysql
from pyspark.sql import SparkSession
from pyspark.sql.functions import col
import logging
import os
import tempfile

logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)


def load_config(config_path='config.json'):
    """从配置文件加载数据库连接参数"""
    with open(config_path, 'r', encoding='utf-8') as f:
        config = json.load(f)
    target = config.get('target', {})
    driver_jar = config.get('driver_jar', None)
    if driver_jar and 'driver_jar' not in target:
        target['driver_jar'] = driver_jar
    return target


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


def init_spark(target_config):
    """初始化 Spark Session"""
    if os.name == 'nt':
        hadoop_home = os.path.join(tempfile.gettempdir(), 'hadoop')
        bin_dir = os.path.join(hadoop_home, 'bin')
        os.makedirs(bin_dir, exist_ok=True)
        winutils_path = os.path.join(bin_dir, 'winutils.exe')
        if not os.path.exists(winutils_path):
            bat_path = os.path.join(bin_dir, 'winutils.bat')
            with open(bat_path, 'w') as f:
                f.write('@echo off\nexit /b 0\n')
            logger.info(f"Windows环境：创建Hadoop目录: {hadoop_home}")
            logger.warning("提示：如果仍然报错，请下载winutils.exe并放到: " + bin_dir)
        os.environ['HADOOP_HOME'] = hadoop_home
        os.environ['hadoop.home.dir'] = hadoop_home
    
    spark_builder = SparkSession.builder \
        .appName("Switch Court") \
        .config("spark.sql.execution.arrow.pyspark.enabled", "true") \
        .config("spark.sql.warehouse.dir", os.path.join(tempfile.gettempdir(), "spark-warehouse")) \
        .config("spark.hadoop.fs.defaultFS", "file:///") \
        .master("local[*]")
    
    driver_jar = target_config.get('driver_jar')
    if driver_jar:
        spark_builder = spark_builder.config("spark.jars", driver_jar)
        logger.info(f"使用驱动jar: {driver_jar}")
    else:
        logger.warning("未指定MySQL驱动jar路径，如果出现驱动找不到的错误，请在config.json中添加driver_jar配置")
    
    return spark_builder.getOrCreate()


def read_user_numbers(file_path):
    """读取用户手机号列表"""
    with open(file_path, 'r', encoding='utf-8') as f:
        numbers = [line.strip() for line in f if line.strip()]
    logger.info(f"读取到 {len(numbers)} 个用户手机号")
    return numbers


def main():
    """主流程：修改用户充值记录和课程记录的校区"""
    script_dir = os.path.dirname(os.path.abspath(__file__))
    user_file = os.path.join(script_dir, 'switch_user.txt')
    config_path = os.path.join(script_dir, 'config.json')
    
    if not os.path.exists(user_file):
        logger.error(f"用户文件不存在: {user_file}")
        sys.exit(1)
    
    target_config = load_config(config_path)
    logger.info("配置文件加载成功")
    
    user_numbers = read_user_numbers(user_file)
    if not user_numbers:
        logger.error("未找到任何用户手机号")
        sys.exit(1)
    
    spark = init_spark(target_config)
    
    try:
        target_url = get_jdbc_url(target_config)
        target_props = get_jdbc_properties(target_config)
        
        # 读取相关表
        logger.info("读取数据库表...")
        prepaid_card_df = spark.read.jdbc(
            url=target_url,
            table='prepaid_card',
            properties=target_props
        ).filter(col('deleted_at').isNull())
        
        charge_df = spark.read.jdbc(
            url=target_url,
            table='charge',
            properties=target_props
        ).filter(col('deleted_at').isNull())
        
        course_df = spark.read.jdbc(
            url=target_url,
            table='course',
            properties=target_props
        ).filter(col('deleted_at').isNull())
        
        course_member_df = spark.read.jdbc(
            url=target_url,
            table='course_member',
            properties=target_props
        )
        
        court_df = spark.read.jdbc(
            url=target_url,
            table='court',
            properties=target_props
        ).filter(col('deleted_at').isNull())
        
        # 创建临时视图
        prepaid_card_df.createOrReplaceTempView('prepaid_card_temp')
        charge_df.createOrReplaceTempView('charge_temp')
        course_df.createOrReplaceTempView('course_temp')
        course_member_df.createOrReplaceTempView('course_member_temp')
        court_df.createOrReplaceTempView('court_temp')
        
        # 根据手机号找到用户ID
        numbers_str = "', '".join(user_numbers)
        user_ids_df = spark.sql(f"""
            SELECT id, number, name
            FROM prepaid_card_temp
            WHERE number IN ('{numbers_str}')
        """)
        
        user_ids = [row.id for row in user_ids_df.collect()]
        if not user_ids:
            logger.warning("未找到任何匹配的用户")
            return
        
        logger.info(f"找到 {len(user_ids)} 个用户ID: {user_ids}")
        
        # 查找"领馆"的court_id
        lingguan_court = spark.sql("""
            SELECT id FROM court_temp WHERE name = '领馆国际校区'
        """).collect()
        
        if not lingguan_court:
            logger.error("未找到'领馆'校区")
            return
        
        lingguan_court_id = lingguan_court[0].id
        logger.info(f"'领馆'校区ID: {lingguan_court_id}")
        
        # 使用pymysql执行更新
        connection = get_connection(target_config)
        
        try:
            with connection.cursor() as cursor:
                # 1. 更新充值记录：将2025年的充值记录的court字段从"雅居乐"改为"领馆"
                user_ids_str = ','.join(map(str, user_ids))
                charge_update_df = spark.sql(f"""
                    SELECT id, prepaid_card_id, court, charged_time
                    FROM charge_temp
                    WHERE prepaid_card_id IN ({user_ids_str})
                      AND court = '雅居乐校区'
                      AND YEAR(charged_time) = 2025
                """)
                
                charge_updates = charge_update_df.collect()
                if charge_updates:
                    logger.info(f"找到 {len(charge_updates)} 条需要更新的充值记录")
                    charge_ids = [row.id for row in charge_updates]
                    charge_ids_str = ','.join(map(str, charge_ids))
                    
                    cursor.execute(f"""
                        UPDATE charge 
                        SET court = '领馆国际校区' 
                        WHERE id IN ({charge_ids_str})
                    """)
                    connection.commit()
                    logger.info(f"成功更新 {len(charge_updates)} 条充值记录")
                else:
                    logger.info("未找到需要更新的充值记录")
                
                # 2. 更新课程记录：查找这些用户2025年参加的课程，如果课程校区是"雅居乐"，换为"领馆"
                course_update_df = spark.sql(f"""
                    SELECT DISTINCT c.id, c.court_id, c.start_time
                    FROM course_temp c
                    INNER JOIN course_member_temp cm ON c.id = cm.course_id
                    INNER JOIN court_temp ct ON c.court_id = ct.id
                    WHERE cm.member_id IN ({user_ids_str})
                      AND ct.name = '雅居乐校区'
                      AND YEAR(c.start_time) = 2025
                """)
                
                course_updates = course_update_df.collect()
                if course_updates:
                    logger.info(f"找到 {len(course_updates)} 条需要更新的课程记录")
                    course_ids = [row.id for row in course_updates]
                    course_ids_str = ','.join(map(str, course_ids))
                    
                    cursor.execute(f"""
                        UPDATE course 
                        SET court_id = {lingguan_court_id} 
                        WHERE id IN ({course_ids_str})
                    """)
                    connection.commit()
                    logger.info(f"成功更新 {len(course_updates)} 条课程记录")
                else:
                    logger.info("未找到需要更新的课程记录")
        
        finally:
            connection.close()
        
        logger.info("数据迁移完成")
        
    except Exception as e:
        logger.error(f"处理失败: {e}")
        import traceback
        logger.error(traceback.format_exc())
        raise
    finally:
        spark.stop()


if __name__ == "__main__":
    main()

