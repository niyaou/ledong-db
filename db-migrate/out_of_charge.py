#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
统计 prepaid_card 表的欠费总数
"""

import json
from pyspark.sql import SparkSession
from pyspark.sql.functions import col, abs as spark_abs, sum as spark_sum, to_date, lit
import logging
import os
import tempfile
import math

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
        .appName("Out of Charge Statistics") \
        .config("spark.sql.execution.arrow.pyspark.enabled", "false") \
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
    """主流程：统计欠费总数"""
    config_path = os.path.join(os.path.dirname(__file__), 'config.json')
    
    source_config, target_config, filter_expired_times = load_config(config_path)
    logger.info("配置文件加载成功")
    if filter_expired_times:
        logger.info("已启用过期次数卡过滤：将排除 times_expire_time 为空或早于 2025-01-01 的记录")
    
    spark = init_spark(target_config)
    
    try:
        target_url = get_jdbc_url(target_config)
        target_props = get_jdbc_properties(target_config)
        
        logger.info("开始读取 prepaid_card 表数据")
        df = spark.read.jdbc(
            url=target_url,
            table='prepaid_card',
            properties=target_props
        )
        
        logger.info("计算总余额并筛选欠费记录")
        df_with_balance = df.withColumn(
            'total_balance',
            col('rest_charge') + (col('annual_count') * 150) + (col('times_count') * 200)
        )
        
        df_debt = df_with_balance.filter(col('total_balance') < 0)
        
        if filter_expired_times:
            logger.info("应用过期次数卡过滤条件")
            df_debt = df_debt.filter(
                col('times_expire_time').isNotNull() & 
                (to_date(col('times_expire_time')) >= to_date(lit('2025-01-01')))
            )
        
        logger.info("计算欠费总数（绝对值之和）")
        result = df_debt.agg(spark_sum(spark_abs(col('total_balance'))).alias('total_debt')).collect()
        
        total_debt = int(result[0]['total_debt']) if result and result[0]['total_debt'] is not None else 0
        
        logger.info("=" * 50)
        logger.info(f"欠费总数: {total_debt}")
        logger.info("=" * 50)
        
        logger.info("按 Court 分组统计欠费")
        df_debt_with_abs = df_debt.withColumn('debt_amount', spark_abs(col('total_balance')))
        
        court_summary = df_debt_with_abs.groupBy('court').agg(
            spark_sum('debt_amount').alias('total_debt')
        ).orderBy('court').collect()
        
        output_lines = []
        output_lines.append("欠费统计报告")
        output_lines.append("=" * 80)
        output_lines.append(f"\n欠费总数: {total_debt}\n")
        output_lines.append("=" * 80)
        output_lines.append("按 Court 分组的欠费统计")
        output_lines.append("=" * 80)
        
        for row in court_summary:
            court = row['court'] if row['court'] else '(空)'
            court_total = int(row['total_debt']) if row['total_debt'] else 0
            output_lines.append(f"\nCourt: {court}")
            output_lines.append(f"  欠费总数: {court_total}")
            output_lines.append(f"  欠费明细:")
            
            court_debt_details = df_debt_with_abs.filter(
                (col('court') == row['court']) if row['court'] else col('court').isNull()
            ).select(
                col('name'),
                col('debt_amount').alias('欠费数量'),
                col('times_expire_time'),
                col('rest_charge'),
                col('annual_count'),
                col('times_count')
            ).orderBy(col('debt_amount').desc()).collect()
            
            for detail in court_debt_details:
                name = detail['name'] if detail['name'] else '(空)'
                debt = math.ceil(detail['欠费数量']) if detail['欠费数量'] else 0
                expire_time = detail['times_expire_time'] if detail['times_expire_time'] else '(空)'
                
                debt_items = []
                rest_charge = detail['rest_charge'] if detail['rest_charge'] is not None else 0.0
                annual_count = detail['annual_count'] if detail['annual_count'] is not None else 0.0
                times_count = detail['times_count'] if detail['times_count'] is not None else 0.0
                
                if rest_charge < 0:
                    debt_items.append(f"余额欠费: {math.ceil(abs(rest_charge))}")
                if annual_count < 0:
                    debt_items.append(f"年卡欠费: {math.ceil(abs(annual_count))}次")
                if times_count < 0:
                    debt_items.append(f"次卡欠费: {math.ceil(abs(times_count))}次")
                
                debt_info = ", ".join(debt_items) if debt_items else "未知"
                output_lines.append(f"    - {name}: {debt} (欠费项目: {debt_info}), 过期时间: {expire_time}")
        
        output_lines.append("\n" + "=" * 80 + "\n")
        
        output_file = os.path.join(os.path.dirname(__file__), 'out_of_charge_result.txt')
        with open(output_file, 'w', encoding='utf-8') as f:
            f.write('\n'.join(output_lines))
        
        logger.info(f"统计结果已保存到: {output_file}")
        print(f"\n统计结果已保存到: {output_file}\n")
        
    except Exception as e:
        logger.error(f"统计失败: {e}")
        import traceback
        logger.error(traceback.format_exc())
        raise
    finally:
        spark.stop()


if __name__ == "__main__":
    main()

