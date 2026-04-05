# JG0 - Go 微服务框架

一个用 Go 从零实现的轻量级微服务框架，包含 HTTP 路由、中间件、ORM、RPC、服务注册发现、协程池、日志、JWT 认证等核心模块。

## 模块说明

| 模块 | 路径 | 说明 |
|------|------|------|
| HTTP 引擎 | `engine/` | 路由、中间件、Context、网关 |
| 简版引擎 | 根包 `msgo` | 另一套精简 HTTP 引擎实现 |
| ORM | `orm/` | 基于反射的轻量 ORM |
| RPC | `rpc/` | gRPC、HTTP RPC、TCP RPC |
| 服务注册 | `register/` | Etcd、Nacos 注册中心 |
| 协程池 | `mspool/` | Goroutine Pool |
| 日志 | `mslog/` | 支持 JSON/Text 格式、文件轮转 |
| JWT | `token/` | JWT 认证中间件 |
| 配置 | `config/` | TOML 配置文件解析 |

---

## 快速开始

### 环境要求

- Go 1.19+
- （可选）Etcd 或 Nacos（使用服务注册功能时需要）
- （可选）MySQL 或其他数据库（使用 ORM 时需要）

### 安装依赖

```bash
go mod tidy
```

### 1. 启动一个 HTTP 服务（engine 包）

```go
package main

import (
    "github.com/ErizJ/JG0/engine"
    "net/http"
)

func main() {
    e := engine.Default()

    // 注册路由
    e.Get("/hello", func(ctx *engine.Context) {
        ctx.Json(map[string]string{"msg": "hello world"})
    })

    e.Post("/user", func(ctx *engine.Context) {
        var user struct {
            Name string `json:"name"`
            Age  int    `json:"age"`
        }
        if err := ctx.BindJson(&user); err != nil {
            ctx.JsonWithStatus(http.StatusBadRequest, map[string]string{"error": err.Error()})
            return
        }
        ctx.Json(user)
    })

    // 启动服务，监听 :8080
    e.Run(":8080")
}
```

运行：
```bash
go run main.go
```

访问：
```bash
curl http://localhost:8080/hello
```

### 2. 使用中间件

```go
e := engine.Default()

// 全局中间件（日志 + panic 恢复）
e.Use(engine.Logging, engine.Recovery)

// 路由级别中间件
e.Get("/protected", func(ctx *engine.Context) {
    ctx.Json(map[string]string{"msg": "ok"})
}, myAuthMiddleware)
```

### 3. 路由分组（根包 msgo）

```go
package main

import (
    msgo "github.com/ErizJ/JG0"
    "net/http"
)

func main() {
    engine := msgo.Default()

    // 路由分组
    userGroup := engine.router.Group("user")
    userGroup.Get("/list", func(ctx *msgo.Context) {
        ctx.JSON(http.StatusOK, []string{"Alice", "Bob"})
    })
    userGroup.Post("/add", func(ctx *msgo.Context) {
        ctx.JSON(http.StatusOK, map[string]string{"msg": "added"})
    })

    engine.Run(":8080")
}
```

### 4. 启用 HTTPS

```go
// 需要证书文件
e.Run(":443", "cert.pem", "key.pem")
```

### 5. 使用 ORM

```go
package main

import (
    "github.com/ErizJ/JG0/orm"
    _ "github.com/go-sql-driver/mysql"
    "fmt"
)

type User struct {
    Id   int64  `msorm:"id"`
    Name string `msorm:"name"`
    Age  int    `msorm:"age"`
}

func main() {
    db, err := orm.Open("mysql", "root:password@tcp(127.0.0.1:3306)/testdb?charset=utf8")
    if err != nil {
        panic(err)
    }

    session := db.New()

    // 插入
    user := &User{Name: "Alice", Age: 18}
    id, affected, err := session.Table("user").Insert(user)
    fmt.Println("insert id:", id, "affected:", affected, "err:", err)

    // 查询
    results, err := session.Table("user").Where("age", 18).Select(&User{})
    fmt.Println("results:", results, "err:", err)

    // 更新
    affected, err = session.Table("user").Where("id", 1).Update("name", "Bob")
    fmt.Println("affected:", affected, "err:", err)

    // 删除
    err = session.Table("user").Where("id", 1).Delete()
    fmt.Println("delete err:", err)

    // 事务
    session.Begin()
    _, _, err = session.Table("user").Insert(&User{Name: "Tx User", Age: 20})
    if err != nil {
        session.Rollback()
    } else {
        session.Commit()
    }
}
```

> 注意：需要手动引入数据库驱动，例如 MySQL：`go get github.com/go-sql-driver/mysql`

### 6. 使用 JWT 认证

```go
package main

import (
    "github.com/ErizJ/JG0/engine"
    "github.com/ErizJ/JG0/token"
    "time"
)

func main() {
    e := engine.Default()

    jwt := &token.JwtHandler{
        Key:            []byte("my-secret-key"),
        Timeout:        time.Hour,
        RefreshTimeout: time.Hour * 24,
        Authenticator: func(ctx *engine.Context) (map[string]any, error) {
            // 验证用户名密码，返回 claims
            return map[string]any{"userId": 1}, nil
        },
    }

    // 登录接口
    e.Post("/login", func(ctx *engine.Context) {
        resp, err := jwt.LoginHandler(ctx)
        if err != nil {
            ctx.JsonWithStatus(401, map[string]string{"error": err.Error()})
            return
        }
        ctx.Json(resp)
    })

    // 需要认证的接口
    e.Get("/profile", func(ctx *engine.Context) {
        claims, _ := ctx.Get("claims")
        ctx.Json(claims)
    }, jwt.AuthInterceptor)

    e.Run(":8080")
}
```

### 7. 使用协程池

```go
package main

import (
    "fmt"
    "github.com/ErizJ/JG0/mspool"
)

func main() {
    pool, err := mspool.NewPool(10) // 容量为 10 的协程池
    if err != nil {
        panic(err)
    }
    defer pool.Release()

    for i := 0; i < 100; i++ {
        n := i
        pool.Submit(func() {
            fmt.Printf("task %d executed\n", n)
        })
    }
}
```

### 8. 使用 TCP RPC

**服务端：**

```go
package main

import (
    "github.com/ErizJ/JG0/rpc/tcp"
    "fmt"
)

type HelloService struct{}

func (h *HelloService) SayHello(name string) string {
    return fmt.Sprintf("Hello, %s!", name)
}

func main() {
    server := tcp.NewTcpServer("127.0.0.1", 9000)
    server.Register("HelloService", &HelloService{})
    server.Run()
}
```

### 9. 使用 Etcd 服务注册

```go
package main

import (
    "github.com/ErizJ/JG0/register"
    "time"
)

func main() {
    // 注册服务
    err := register.EtcdRegisterService(register.Option{
        Endpoints:   []string{"localhost:2379"},
        DialTimeout: 5 * time.Second,
        ServiceName: "user-service",
        Host:        "127.0.0.1",
        Port:        8080,
    })
    if err != nil {
        panic(err)
    }

    // 查找服务
    addr, err := register.GetEtcdValue(register.Option{
        Endpoints:   []string{"localhost:2379"},
        DialTimeout: 5 * time.Second,
        ServiceName: "user-service",
    })
    if err != nil {
        panic(err)
    }
    _ = addr // "127.0.0.1:8080"
}
```

### 10. 配置文件

在项目根目录创建 `config/app.toml`：

```toml
[log]
path = "logs"
```

启动时通过命令行参数指定：

```bash
go run main.go -conf config/app.toml
```

---

## 项目结构

```
JG0/
├── engine/          # 主 HTTP 引擎（推荐使用）
│   ├── engine.go    # Engine 核心，路由分发，网关
│   ├── router.go    # 路由注册
│   ├── context.go   # 请求/响应上下文
│   ├── middleware.go # 内置中间件（Logging、Recovery）
│   ├── auth.go      # 认证工具
│   ├── treeNode.go  # 前缀树路由
│   └── gateway/     # 反向代理网关
├── ms.go            # 简版 HTTP 引擎（根包）
├── context.go
├── tree.go
├── orm/             # 轻量 ORM
├── rpc/             # RPC（gRPC、HTTP、TCP）
├── register/        # 服务注册（Etcd、Nacos）
├── mspool/          # Goroutine 协程池
├── mslog/           # 日志系统
├── token/           # JWT 认证
├── config/          # TOML 配置
├── binding/         # 请求体绑定（JSON/XML）
├── render/          # 响应渲染（JSON/XML/HTML/String）
├── validator/       # 结构体参数校验
├── mserror/         # 自定义错误处理
└── internal/        # 内部工具库
```

---

## 运行测试

```bash
# 运行所有测试
go test ./...

# 运行协程池测试
go test ./mspool/...

# 运行 ORM 测试（需要数据库连接）
go test ./orm/...
```

---

## 构建

```bash
go build ./...
```
