# Ledong DB - Golang 云后台服务

基于 Gin + GORM + MySQL 的云后台服务，集成腾讯云短信 SDK。

## 技术栈

- Go 1.25
- Gin - Web 框架
- GORM - ORM 框架
- MySQL - 数据库
- 腾讯云 SMS SDK - 短信服务
- bigcache - 内存缓存
- gorm-cache - GORM 缓存插件

## 项目结构

```
ledong-db/
├── cmd/server/          # 应用入口
├── internal/            # 内部包
│   ├── config/          # 配置管理
│   ├── database/        # 数据库连接
│   ├── model/           # GORM 实体
│   ├── cache/           # 缓存封装
│   ├── handler/         # HTTP 处理器
│   ├── service/         # 业务逻辑
│   └── router/          # 路由配置
└── pkg/                 # 公共包
    └── tencent/         # 腾讯云 SDK 封装
```

## 环境变量

## 更新mod版本
```
go get -u all
go mod tidy
```



## 运行

```bash
go run cmd/server/main.go
```

## 生成swagger
```
cd d:\project\ledong-db; $env:PATH += ";$env:GOPATH\bin"; swag init -g cmd/server/main.go -o docs
```

###
#### 使用方式
#### 启动服务后，访问 Swagger UI：
   http://localhost:8080/swagger/index.html
#### 更新 API 文档：
   swag init -g cmd/server/main.go -o docs
#### 查看生成的 OpenAPI 规范：
JSON: docs/swagger.json
YAML: docs/swagger.yaml

## 业务公式说明

### 教练效率指标（Analyse）

> 





单节课成员数的计算规则：
**总成员数**：每节课的成员数加起来的总和。
**效率指标 = 总成员数 ÷ 有效课程数**
**有效课程数**：有消费记录的课程数量。只要有学员消费（刷卡、扣次、扣余额），这节课就算一个有效课程。
| 消费情况                | 班课            | 私教 |
|:--------:              |:----:           |:----:|
| 多人消费（大于1人）      | 按实际消费人次算 | 按实际消费人次算 |
| 只有1人消费             | 算 **1** 人     | 算 **2** 人 |
| 无人消费                | 不计入           | 不计入 |

**举例**：某教练一个月上了四节课——班课来了5人、班课来了1人、私教课A来了1人、私教课B无人消费。


- 总成员数 = 5 + 1 + 2 = 8
- 有效课程数 = 3（私教课B无人消费，不计入）
- 效率指标 = 8 ÷ 3 ≈ **2.67**

> 注：满班率需要"满班容量"作为分母，当前系统未记录该字段，因此使用效率指标代替。