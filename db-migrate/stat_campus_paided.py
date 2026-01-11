#!/usr/bin/env python3
# -*- coding: utf-8 -*-

import json
from collections import defaultdict

def stat_campus_paided(file_path):
    """统计每个校区中status为PAIDED和REFUNDED的记录的total_fee总和"""
    campus_total = defaultdict(float)
    campus_refunded = defaultdict(float)
    campus_paided_count = defaultdict(int)
    campus_refunded_count = defaultdict(int)
    
    with open(file_path, 'r', encoding='utf-8') as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            try:
                record = json.loads(line)
                campus = record.get('campus', '未知校区')
                status = record.get('status')
                
                if status == 'PAIDED':
                    campus_paided_count[campus] += 1
                    total_fee = record.get('total_fee', 0)
                    if isinstance(total_fee, (int, float)):
                        campus_total[campus] += total_fee
                elif status == 'REFUNDED':
                    campus_refunded_count[campus] += 1
                    total_fee = record.get('total_fee', 0)
                    if isinstance(total_fee, (int, float)):
                        campus_refunded[campus] += total_fee
            except json.JSONDecodeError:
                continue
    
    print("各校区PAIDED状态订单total_fee统计：")
    print("-" * 50)
    for campus in sorted(campus_total.keys()):
        print(f"{campus}: {campus_total[campus]:.2f}")
    print("-" * 50)
    print(f"总计: {sum(campus_total.values()):.2f}")
    print()
    
    print("各校区REFUNDED状态订单total_fee统计：")
    print("-" * 50)
    for campus in sorted(campus_refunded.keys()):
        print(f"{campus}: {campus_refunded[campus]:.2f}")
    print("-" * 50)
    print(f"总计: {sum(campus_refunded.values()):.2f}")
    print()
    
    all_campuses = set(campus_paided_count.keys()) | set(campus_refunded_count.keys())
    print("各校区付费订单数减去退款订单数：")
    print("-" * 50)
    for campus in sorted(all_campuses):
        paided_count = campus_paided_count.get(campus, 0)
        refunded_count = campus_refunded_count.get(campus, 0)
        diff = paided_count - refunded_count
        print(f"{campus}: {diff} (付费:{paided_count} - 退款:{refunded_count})")
    print("-" * 50)
    total_paided = sum(campus_paided_count.values())
    total_refunded = sum(campus_refunded_count.values())
    print(f"总计: {total_paided - total_refunded} (付费:{total_paided} - 退款:{total_refunded})")
    print()
    
    all_campuses_fee = set(campus_total.keys()) | set(campus_refunded.keys())
    print("各校区净收入统计：")
    print("-" * 50)
    for campus in sorted(all_campuses_fee):
        paided_fee = campus_total.get(campus, 0)
        refunded_fee = campus_refunded.get(campus, 0)
        net_income = paided_fee - refunded_fee
        print(f"{campus}: {net_income:.2f} (付费:{paided_fee:.2f} - 退款:{refunded_fee:.2f})")
    print("-" * 50)
    total_paided_fee = sum(campus_total.values())
    total_refunded_fee = sum(campus_refunded.values())
    print(f"总计: {total_paided_fee - total_refunded_fee:.2f} (付费:{total_paided_fee:.2f} - 退款:{total_refunded_fee:.2f})")
    print()
    
    total_paided_fee = sum(campus_total.values())
    total_refunded_fee = sum(campus_refunded.values())
    print("总量统计：")
    print("-" * 50)
    print(f"总付费金额: {total_paided_fee:.2f}")
    print(f"总退款金额: {total_refunded_fee:.2f}")
    print(f"净收入: {total_paided_fee - total_refunded_fee:.2f}")
    print(f"总付费订单数: {total_paided}")
    print(f"总退款订单数: {total_refunded}")
    print(f"净订单数: {total_paided - total_refunded}")
    print("-" * 50)

if __name__ == '__main__':
    stat_campus_paided('database_export-sIzUpS4mFDQp.json')

