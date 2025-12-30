#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
查询指定会员从2025-01-01开始的消费记录并导出为Excel
"""

import json
import sys
from pyspark.sql import SparkSession
from pyspark.sql.functions import col, to_date, lit
import logging
import os
import tempfile
import pandas as pd

logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)


def load_config(config_path='config.json'):
    """从配置文件加载数据库连接参数"""
    with open(config_path, 'r', encoding='utf-8') as f:
        config = json.load(f)
    source = config.get('source', {})
    target = config.get('target', {})
    driver_jar = config.get('driver_jar', None)
    if driver_jar and 'driver_jar' not in source:
        source['driver_jar'] = driver_jar
    if driver_jar and 'driver_jar' not in target:
        target['driver_jar'] = driver_jar
    filter_expired_times = config.get('filter_expired_times', False)
    return source, target, filter_expired_times


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
        .appName("Member Spend List") \
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


def main():
    """主流程：查询会员消费记录并导出为Excel"""
    if len(sys.argv) < 2:
        logger.error("请提供prepaid_id参数")
        print("用法: python member_spend_list.py <prepaid_id>")
        sys.exit(1)
    
    try:
        prepaid_id = int(sys.argv[1])
    except ValueError:
        logger.error("prepaid_id必须是数字")
        sys.exit(1)
    
    config_path = os.path.join(os.path.dirname(__file__), 'config.json')
    source_config, target_config, _ = load_config(config_path)
    logger.info("配置文件加载成功")
    
    spark = init_spark(target_config)
    
    try:
        target_url = get_jdbc_url(target_config)
        target_props = get_jdbc_properties(target_config)
        
        logger.info(f"查询会员ID {prepaid_id} 从2025-01-01开始的消费记录")
        
        # 读取spend表并过滤已删除记录
        spend_df = spark.read.jdbc(
            url=target_url,
            table='spend',
            properties=target_props
        ).filter(col('deleted_at').isNull())
        
        # 读取course表并过滤已删除记录
        course_df = spark.read.jdbc(
            url=target_url,
            table='course',
            properties=target_props
        ).filter(col('deleted_at').isNull())
        
        # 读取coach表并过滤已删除记录
        coach_df = spark.read.jdbc(
            url=target_url,
            table='coach',
            properties=target_props
        ).filter(col('deleted_at').isNull())
        
        # 读取court表并过滤已删除记录
        court_df = spark.read.jdbc(
            url=target_url,
            table='court',
            properties=target_props
        ).filter(col('deleted_at').isNull())
        
        # 读取charge表并过滤已删除记录
        charge_df = spark.read.jdbc(
            url=target_url,
            table='charge',
            properties=target_props
        ).filter(col('deleted_at').isNull())
        
        # 关联查询：使用SQL更清晰
        spend_df.createOrReplaceTempView('spend_temp')
        course_df.createOrReplaceTempView('course_temp')
        coach_df.createOrReplaceTempView('coach_temp')
        court_df.createOrReplaceTempView('court_temp')
        charge_df.createOrReplaceTempView('charge_temp')
        
        # 查询消费记录
        spend_result_df = spark.sql(f"""
            SELECT 
                s.id as `消费ID`,
                s.charge as `消费金额`,
                s.times as `次卡消费`,
                s.annual_times as `年卡消费`,
                s.quantities as `数量`,
                s.description as `描述`,
                c.start_time as `课程开始时间`,
                c.end_time as `课程结束时间`,
                c.duration as `课程时长`,
                c.course_type as `课程类型`,
                c.description as `课程描述`,
                coach.name as `教练`,
                court.name as `场地`
            FROM spend_temp s
            INNER JOIN course_temp c ON s.course_id = c.id
            LEFT JOIN coach_temp coach ON c.coach_id = coach.coach_id
            LEFT JOIN court_temp court ON c.court_id = court.id
            WHERE s.prepaid_card_id = {prepaid_id}
              AND DATE(c.start_time) >= DATE('2025-01-01')
            ORDER BY c.start_time DESC
        """)
        
        # 查询充值记录
        charge_result_df = spark.sql(f"""
            SELECT 
                ch.id as `充值ID`,
                ch.charge as `充值金额`,
                ch.times as `次卡充值`,
                ch.annual_times as `年卡充值`,
                ch.worth as `价值`,
                ch.court as `场地`,
                ch.description as `描述`,
                ch.charged_time as `充值时间`,
                coach.name as `教练`
            FROM charge_temp ch
            LEFT JOIN coach_temp coach ON ch.coach_id = coach.coach_id
            WHERE ch.prepaid_card_id = {prepaid_id}
              AND DATE(ch.charged_time) >= DATE('2025-01-01')
            ORDER BY ch.charged_time DESC
        """)
        
        # 转换为pandas DataFrame
        logger.info("正在转换为pandas DataFrame...")
        spend_pandas_df = spend_result_df.toPandas()
        charge_pandas_df = charge_result_df.toPandas()
        
        if len(spend_pandas_df) == 0 and len(charge_pandas_df) == 0:
            logger.warning(f"未找到会员ID {prepaid_id} 从2025-01-01开始的消费和充值记录")
            print(f"\n未找到会员ID {prepaid_id} 从2025-01-01开始的消费和充值记录\n")
            return
        
        logger.info(f"找到 {len(spend_pandas_df)} 条消费记录，{len(charge_pandas_df)} 条充值记录")
        
        # 使用ExcelWriter写入多个sheet
        output_file = os.path.join(os.path.dirname(__file__), f'member_spend_list_{prepaid_id}.xlsx')
        with pd.ExcelWriter(output_file, engine='openpyxl') as writer:
            if len(spend_pandas_df) > 0:
                spend_pandas_df.to_excel(writer, sheet_name='消费记录', index=False)
            if len(charge_pandas_df) > 0:
                charge_pandas_df.to_excel(writer, sheet_name='充值记录', index=False)
        
        logger.info(f"记录已导出到: {output_file}")
        print(f"\n记录已导出到: {output_file}\n")
        print(f"消费记录: {len(spend_pandas_df)} 条\n充值记录: {len(charge_pandas_df)} 条\n")
        
    except Exception as e:
        logger.error(f"查询失败: {e}")
        import traceback
        logger.error(traceback.format_exc())
        raise
    finally:
        spark.stop()


if __name__ == "__main__":
    main()

