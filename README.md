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