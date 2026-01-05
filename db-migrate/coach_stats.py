#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
查询指定教练在领馆国际校区的课程消费和充值记录
"""

import json
import sys
from pyspark.sql import SparkSession
from pyspark.sql.functions import col
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
        .appName("Coach Stats") \
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
    """主流程：查询教练在领馆国际校区的消费和充值记录"""
    if len(sys.argv) < 2:
        logger.error("请提供coach_id参数")
        print("用法: python coach_stats.py <coach_id>")
        sys.exit(1)
    
    try:
        coach_id = int(sys.argv[1])
    except ValueError:
        logger.error("coach_id必须是数字")
        sys.exit(1)
    
    config_path = os.path.join(os.path.dirname(__file__), 'config.json')
    target_config = load_config(config_path)
    logger.info("配置文件加载成功")
    
    spark = init_spark(target_config)
    
    try:
        target_url = get_jdbc_url(target_config)
        target_props = get_jdbc_properties(target_config)
        
        logger.info(f"查询教练ID {coach_id} 在领馆国际校区的消费和充值记录")
        
        # 读取相关表并过滤已删除记录
        spend_df = spark.read.jdbc(
            url=target_url,
            table='spend',
            properties=target_props
        ).filter(col('deleted_at').isNull())
        
        course_df = spark.read.jdbc(
            url=target_url,
            table='course',
            properties=target_props
        ).filter(col('deleted_at').isNull())
        
        court_df = spark.read.jdbc(
            url=target_url,
            table='court',
            properties=target_props
        ).filter(col('deleted_at').isNull())
        
        charge_df = spark.read.jdbc(
            url=target_url,
            table='charge',
            properties=target_props
        ).filter(col('deleted_at').isNull())
        
        prepaid_card_df = spark.read.jdbc(
            url=target_url,
            table='prepaid_card',
            properties=target_props
        ).filter(col('deleted_at').isNull())
        
        # 创建临时视图
        spend_df.createOrReplaceTempView('spend_temp')
        course_df.createOrReplaceTempView('course_temp')
        court_df.createOrReplaceTempView('court_temp')
        charge_df.createOrReplaceTempView('charge_temp')
        prepaid_card_df.createOrReplaceTempView('prepaid_card_temp')
        
        # 查询消费记录：通过course关联，筛选领馆国际校区
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
                court.name as `场地`,
                pc.name as `会员姓名`,
                pc.number as `会员手机号`
            FROM spend_temp s
            INNER JOIN course_temp c ON s.course_id = c.id
            INNER JOIN court_temp court ON c.court_id = court.id
            LEFT JOIN prepaid_card_temp pc ON s.prepaid_card_id = pc.id
            WHERE c.coach_id = {coach_id}
              AND court.name = '领馆国际校区'
            ORDER BY c.start_time DESC
        """)
        
        # 查询充值记录：直接通过charge表的coach_id和court字段筛选
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
                pc.name as `会员姓名`,
                pc.number as `会员手机号`
            FROM charge_temp ch
            LEFT JOIN prepaid_card_temp pc ON ch.prepaid_card_id = pc.id
            WHERE ch.coach_id = {coach_id}
              AND ch.court = '领馆国际校区'
            ORDER BY ch.charged_time DESC
        """)
        
        # 转换为pandas DataFrame
        logger.info("正在转换为pandas DataFrame...")
        spend_pandas_df = spend_result_df.toPandas()
        charge_pandas_df = charge_result_df.toPandas()
        
        if len(spend_pandas_df) == 0 and len(charge_pandas_df) == 0:
            logger.warning(f"未找到教练ID {coach_id} 在领馆国际校区的消费和充值记录")
            print(f"\n未找到教练ID {coach_id} 在领馆国际校区的消费和充值记录\n")
            return
        
        logger.info(f"找到 {len(spend_pandas_df)} 条消费记录，{len(charge_pandas_df)} 条充值记录")
        
        # 使用ExcelWriter写入多个sheet
        output_file = os.path.join(os.path.dirname(__file__), f'coach_stats_{coach_id}.xlsx')
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

