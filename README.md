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

参考 `.env.example` 配置环境变量。

## 运行

```bash
go run cmd/server/main.go
```
