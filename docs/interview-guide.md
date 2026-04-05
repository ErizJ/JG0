# JG0 项目面试指南

> 本文档结合项目源码，系统梳理核心功能实现原理，以及面试官可能提出的问题和标准回答。

---

## 目录

1. [项目整体介绍](#1-项目整体介绍)
2. [HTTP 引擎与路由系统](#2-http-引擎与路由系统)
3. [中间件机制](#3-中间件机制)
4. [Context 上下文设计](#4-context-上下文设计)
5. [对象池（sync.Pool）的使用](#5-对象池syncpool的使用)
6. [ORM 框架实现](#6-orm-框架实现)
7. [协程池实现](#7-协程池实现)
8. [日志系统](#8-日志系统)
9. [JWT 认证中间件](#9-jwt-认证中间件)
10. [RPC 通信](#10-rpc-通信)
11. [服务注册与发现](#11-服务注册与发现)
12. [综合性问题](#12-综合性问题)

---

## 1. 项目整体介绍

### 项目是什么？

**推荐回答：**

> 这是我从零手写的一个 Go 微服务框架，命名为 JG0。整体参考了 Gin 框架的设计思路，核心包含以下模块：HTTP 引擎（前缀树路由 + 中间件链）、轻量 ORM（基于反射）、TCP/gRPC RPC 框架、服务注册发现（支持 Etcd 和 Nacos）、协程池、结构化日志、JWT 认证。
>
> 做这个项目的目的是深入理解 Web 框架底层原理，比如 Gin 的路由为什么快、sync.Pool 如何减少 GC、中间件链是怎么串联的。这些原理在使用框架时往往是黑盒，自己实现一遍就彻底搞清楚了。

---

## 2. HTTP 引擎与路由系统

### 核心实现

路由系统分两层：**前缀树（Trie Tree）存储路径结构**，**HashMap 存储处理函数**。

**前缀树节点结构（`engine/treeNode.go`）：**

```go
type treeNode struct {
    name       string       // 路径片段，如 "user"
    children   []*treeNode  // 子节点
    routerName string       // 完整路由名，如 "/user/list"
    isEnd      bool         // 是否是终止节点
}
```

**注册路由时（Put）：** 将路径按 `/` 拆分，逐段插入树中。

```go
// engine/treeNode.go:17
func (t *treeNode) Put(path string, routerName ...string) {
    pathArr := strings.Split(path, sep)   // "/user/list" -> ["", "user", "list"]
    for i := 1; i < len(pathArr); i++ {
        name := pathArr[i]
        // 查找是否已有该子节点，没有则新建
        isEnd := i == len(pathArr)-1
        node := &treeNode{name: name, isEnd: isEnd, routerName: rName}
        t.children = append(t.children, node)
        t = node
    }
}
```

**匹配路由时（Get）：** 逐段匹配，支持通配符 `*`（单段）和 `**`（多段），以及参数路由 `:id`。

```go
// engine/treeNode.go:46
for _, child := range children {
    if child.name == name || child.name == "*" || strings.Contains(child.name, ":") {
        isMatch = true
        // ...
    }
}
```

**处理函数存储（`engine/router.go`）：**

```go
// handlerFuncMap: 路由名 -> HTTP方法 -> 处理函数
handlerFuncMap map[string]map[string]HandlerFunc
// 例：handlerFuncMap["/user/list"]["GET"] = listHandler
```

**请求分发（`engine/engine.go:ServeHTTP`）：**
1. 用 treeNode.Get 找到匹配节点，获取 routerName
2. 用 routerName + Method 从 handlerFuncMap 取出 handler
3. 包裹中间件后执行

---

### 面试问题与回答

**Q1：你的路由为什么用前缀树而不是直接用 map？**

> map 只能做精确匹配，无法支持 `/user/:id`、`/static/*` 这样的动态路由和通配符路由。前缀树的每个节点代表路径的一段，天然支持：
> - 精确匹配：`/user/list`
> - 参数路由：`/user/:id`（节点名含 `:`）
> - 单段通配：`/static/*`
> - 多段通配：`/static/**`
>
> 时间复杂度是 O(路径深度)，不随路由数量增长。Gin、HttpRouter 都是这个原理。

**Q2：路由注册时怎么防止重复？**

> 在 `engine/router.go:33-35` 注册时会检查：
> ```go
> _, ok = r.handlerFuncMap[name][method]
> if ok {
>     panic(name + " 有重复的路由")
> }
> ```
> 同一路径+方法组合注册两次直接 panic，在应用启动阶段就暴露问题，不会留到运行时。

**Q3：动态路由的参数怎么取出来？**

> 在 treeNode.Get 匹配时，如果节点名含 `:`（如 `:id`），会匹配实际请求路径中对应位置的值。可以通过 `ctx.R.URL.Path` 和路由模板做对比解析出参数值。这部分在当前实现中可以进一步完善，加入参数提取逻辑存入 Context。

---

## 3. 中间件机制

### 核心实现

中间件采用**洋葱模型**，类型定义：

```go
// engine/middleware.go:17
type MiddlewareFunc func(handlerFunc HandlerFunc) HandlerFunc
```

每个中间件接收下一个 handler，返回一个新的 handler，形成嵌套调用链。

**执行流程（`engine/engine.go:89-105`）：**

```go
func (e *Engine) methodHandler(h HandlerFunc, ctx *Context) {
    // 1. 先包裹引擎级中间件（全局）
    for _, middleware := range e.middlewares {
        h = middleware(h)
    }
    // 2. 再包裹路由级中间件
    funcMiddles := e.router.middlewaresFuncMap[ctx.NodeRouterName][ctx.RequestMethod]
    if funcMiddles != nil {
        funcLen := len(funcMiddles) - 1
        for i := funcLen; i > -1; i-- {  // 注意：路由级是逆序包裹
            middleware := funcMiddles[i]
            h = middleware(h)
        }
    }
    // 3. 执行最终 handler
    h(ctx)
}
```

**以 Logging 中间件为例（`engine/middleware.go:19-53`）：**

```go
func Logging(next HandlerFunc) HandlerFunc {
    return func(ctx *Context) {
        start := time.Now()
        next(ctx)                          // 执行业务逻辑
        latency := time.Now().Sub(start)   // 计算耗时
        // 打印日志
    }
}
```

**Recovery 中间件（`engine/middleware.go:63-80`）：**

```go
func Recovery(next HandlerFunc) HandlerFunc {
    return func(ctx *Context) {
        defer func() {
            if err := recover(); err != nil {
                // 区分 MsError（业务错误）和普通 panic
                if e, ok := err.(error); ok {
                    var msError *mserror.MsError
                    if errors.As(e, &msError) {
                        msError.ExecResult()   // 执行用户自定义错误处理
                        return
                    }
                }
                // 打印堆栈，返回 500
                ctx.Fail(http.StatusInternalServerError, "Internal Server Error")
            }
        }()
        next(ctx)
    }
}
```

---

### 面试问题与回答

**Q4：你的中间件是怎么实现的？和 Gin 有什么区别？**

> 我用的是**函数包裹（wrapper）模式**，也叫洋葱模型。每个中间件接收 next handler 返回一个新 handler，最终形成一个函数嵌套链：
>
> ```
> Logging(Recovery(Auth(actualHandler)))
> ```
>
> 请求进来时从外到内执行前置逻辑，handler 执行完后从内到外执行后置逻辑。
>
> Gin 用的是另一种方式：在 Context 里维护一个 `[]HandlerFunc` 数组和 `index` 指针，调用 `c.Next()` 推进 index，不用函数嵌套。两种方式效果相同，我选的方式更直观，Gin 的方式在性能上略好（不产生闭包）。

**Q5：如何实现一个鉴权中间件？**

> 以 JWT 鉴权为例，就是在中间件里解析 Header 里的 token，验证通过后把 claims 存进 Context，业务 handler 直接从 Context 取：
> ```go
> func AuthMiddleware(next HandlerFunc) HandlerFunc {
>     return func(ctx *Context) {
>         token := ctx.R.Header.Get("Authorization")
>         claims, err := parseToken(token)
>         if err != nil {
>             ctx.W.WriteHeader(401)
>             return           // 不调用 next，请求被拦截
>         }
>         ctx.Set("claims", claims)
>         next(ctx)            // 验证通过，继续执行
>     }
> }
> ```

**Q6：Recovery 怎么区分业务错误和系统 panic？**

> 项目里定义了 `MsError` 结构体（`mserror/errors.go`）。业务逻辑里用 `msError.Put(err)` 主动触发 panic 时，panic 的值是 `*MsError` 类型。Recovery 在 `recover()` 后用 `errors.As` 判断是否是 `*MsError`，如果是就调用用户注册的 `errorFuc` 回调做自定义处理（比如返回格式化的 JSON 错误）；如果是普通 panic（如数组越界）就打印堆栈并返回 500。

---

## 4. Context 上下文设计

### 核心实现

Context 是每次 HTTP 请求的核心对象，封装了请求读取和响应写入的所有操作：

```go
// engine/context.go:23-38
type Context struct {
    W              http.ResponseWriter
    R              *http.Request
    NodeRouterName string
    RequestMethod  string
    engine         *Engine
    queryCache     url.Values       // URL 参数缓存，避免重复解析
    StatusCode     int
    Logger         *msLog.Logger
    Keys           map[string]any   // 中间件间数据传递（线程安全）
    mu             sync.RWMutex     // 保护 Keys 的读写锁
    sameSite       http.SameSite
}
```

**参数获取有缓存优化：**

```go
// engine/context.go:200
func (c *Context) initQueryCache() {
    if c.queryCache == nil {              // 只在第一次调用时解析
        if c.R != nil {
            c.queryCache = c.R.URL.Query()
        }
    }
}
```

**中间件间数据传递（线程安全）：**

```go
func (c *Context) Set(key string, data any) {
    c.mu.Lock()
    if c.Keys == nil {
        c.Keys = make(map[string]any, 1)
    }
    c.Keys[key] = data
    c.mu.Unlock()
}

func (c *Context) Get(key string) (value any, ok bool) {
    c.mu.RLock()
    value, ok = c.Keys[key]
    c.mu.RUnlock()
    return
}
```

**统一渲染层（`engine/context.go:295-300`）：**

```go
func (c *Context) Render(r render.Render, statusCode int) error {
    c.StatusCode = statusCode
    r.WriteContentType(c.W)       // 先写 Content-Type header
    c.W.WriteHeader(statusCode)   // 再写状态码
    return r.Render(c.W)          // 最后写 body
}
```

所有响应方法（JSON、XML、HTML、String）都走这个统一入口，保证 header 写入顺序正确（HTTP 规范要求 Header 必须在 Body 之前写入）。

---

### 面试问题与回答

**Q7：Context 里的 Keys map 为什么要加读写锁？**

> 虽然一个请求对应一个 goroutine，但中间件可能异步操作 Context（比如启动子 goroutine 做日志上报），此时会有并发读写 Keys 的情况。用 `sync.RWMutex`：读操作（Get）可以并发，写操作（Set）互斥，比全用 Mutex 性能更好。

**Q8：响应 JSON 的流程是什么？**

> 调用 `ctx.Json(data)` → 进入 `Render(&render.JSON{Data: data}, 200)` → 先调 `WriteContentType` 写 `Content-Type: application/json` → 再 `WriteHeader(200)` → 最后 `json.NewEncoder(w).Encode(data)` 写 body。
>
> 关键点是顺序：Go 的 `http.ResponseWriter` 一旦开始写 body，就无法再修改 header，所以必须先写 Content-Type 再写 body。

---

## 5. 对象池（sync.Pool）的使用

### 核心实现

每次 HTTP 请求都需要创建一个 Context 对象。高并发下频繁分配和回收对象会给 GC 带来很大压力。用 `sync.Pool` 复用 Context：

```go
// ms.go:130-132
engine.pool.New = func() any {
    return engine.allocateContext()   // 池里没有时才新建
}

// ms.go:ServeHTTP
func (e *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    ctx := e.pool.Get().(*Context)    // 从池里取
    ctx.W = w
    ctx.R = r
    ctx.Keys = nil                    // 重置状态，防止数据污染
    ctx.queryCache = nil
    ctx.StatusCode = 0
    e.httpRequestHandle(ctx)
    e.pool.Put(ctx)                   // 用完放回池里
}
```

---

### 面试问题与回答

**Q9：sync.Pool 有什么特点？为什么适合这个场景？**

> `sync.Pool` 有几个关键特性：
> 1. **GC 时对象会被回收**：Pool 里的对象在 GC 时可能被清空，所以不能用来存需要持久化的对象，只适合临时复用。
> 2. **无法控制池大小**：不像数据库连接池可以设上限，sync.Pool 大小由 GC 决定。
> 3. **并发安全**：内部有 per-P（per-processor）的本地列表，减少锁竞争。
>
> 适合 Context 的原因：Context 生命周期恰好是单次请求，用完就放回，不需要持久化，是 sync.Pool 的理想场景。Gin 框架也是完全一样的做法。
>
> **关键点**：放回池之前必须清零状态（Keys、queryCache 等），否则上一个请求的数据会泄漏给下一个请求，造成安全漏洞。这也是我在代码里专门修复的一个 bug。

**Q10：sync.Pool 和自己实现的 Worker Pool（mspool）有什么区别？**

> 完全不同的概念：
> - `sync.Pool` 是**对象复用池**，目的是减少内存分配，对象随时可能被 GC 回收。
> - `mspool.Pool` 是**协程池**，目的是控制并发 goroutine 数量，防止 goroutine 泄漏和内存耗尽。
>
> 协程池控制的是并发度，对象池控制的是内存分配频率，两者解决的是不同问题。

---

## 6. ORM 框架实现

### 核心实现

ORM 的核心是**反射**：通过 `reflect` 包在运行时读取结构体的字段名、字段类型、Tag，动态生成 SQL 语句。

**Insert 的实现（`orm/orm.go:fieldNames`）：**

```go
func (s *MsSession) fieldNames(data any) {
    t := reflect.TypeOf(data)   // 获取类型信息
    v := reflect.ValueOf(data)  // 获取值信息
    tVar := t.Elem()            // 解引用指针
    vVar := v.Elem()

    for i := 0; i < tVar.NumField(); i++ {
        field := tVar.Field(i)
        // 优先读 mssql tag，没有则用字段名转蛇形
        sqlTag := field.Tag.Get("mssql")
        if sqlTag == "" {
            sqlTag = strings.ToLower(Name(field.Name))  // UserName -> user_name
        }
        // id 字段且值<=0，认为是自增，跳过
        if sqlTag == "id" && isAutoId(vVar.Field(i).Interface()) {
            continue
        }
        fieldNames = append(fieldNames, sqlTag)
        placeholder = append(placeholder, "?")
        values = append(values, vVar.Field(i).Interface())
    }
    // 生成：INSERT INTO user (name, age) VALUES (?, ?)
}
```

**驼峰转蛇形（`orm/orm.go:Name`）：**

```go
func Name(name string) string {
    // "UserName" -> "user_name"
    // 遇到大写字母插入 "_"
    for index, value := range all {
        if value >= 65 && value <= 90 {  // A-Z
            sb.WriteString(name[lastIndex:index])
            sb.WriteString("_")
            lastIndex = index
        }
    }
    sb.WriteString(name[lastIndex:])
    return sb.String()
}
```

**链式调用设计：**

```go
// 每个方法返回 *MsSession 自身，支持链式调用
session.Table("user").Where("age", 18).Select(&User{})
```

**事务支持：**

```go
func (s *MsSession) Begin() error {
    tx, err := s.db.db.Begin()
    s.tx = tx
    s.beginTx = true
    return nil
}
// Commit 和 Rollback 都先检查 s.tx != nil && s.beginTx
```

---

### 面试问题与回答

**Q11：ORM 是怎么把结构体映射到数据库表的？**

> 核心是 Go 的 `reflect` 包。Insert 时，通过 `reflect.TypeOf(data).Elem()` 遍历结构体的每个字段，读取字段名和 `mssql` tag。如果没有 tag，就把字段名做驼峰转蛇形（`UserName` → `user_name`）作为列名。然后用 `reflect.ValueOf(data).Elem().Field(i).Interface()` 获取每个字段的实际值，动态拼接成 `INSERT INTO user (name, age) VALUES (?, ?)` 这样的 SQL，再用 `db.Prepare + stmt.Exec` 执行参数化查询，防止 SQL 注入。

**Q12：为什么用参数化查询而不是字符串拼接 SQL？**

> 字符串拼接有 SQL 注入风险。比如用户输入 `' OR 1=1 --`，拼接后就变成 `WHERE name = '' OR 1=1 --'`，可以绕过条件查询所有数据。
>
> 参数化查询（Prepared Statement）把 SQL 结构和数据分离，`?` 占位符的值由数据库驱动单独处理，不会被解释为 SQL 语法，从根本上杜绝注入。

**Q13：reflect 有什么性能问题？怎么优化？**

> 反射比直接调用慢 10-100 倍，主要开销在：类型信息的动态查找、接口装箱拆箱、无法内联优化。
>
> 优化思路：
> 1. **缓存反射结果**：同一个结构体的字段信息每次都 reflect 很浪费，可以用 `sync.Map` 缓存 `reflect.Type` 的字段映射。
> 2. **代码生成**：像 GORM v2、sqlx 等框架在编译时生成代码，完全避免运行时反射。
> 3. **unsafe 指针**：直接操作内存，跳过反射，极限性能优化（风险高）。
>
> 在这个项目里 ORM 是学习用途，以理解原理为主，性能不是首要考量。

---

## 7. 协程池实现

### 核心实现

协程池解决的问题：如果每个任务都 `go func()`，任务量大时 goroutine 数量无上限，会耗尽内存。

**数据结构（`mspool/pool.go`）：**

```go
type Pool struct {
    cap         int32        // 最大 worker 数量
    running     int32        // 当前运行的 worker 数（原子操作）
    workers     []*Worker    // 空闲 worker 队列
    expire      time.Duration
    release     chan signal  // 关闭信号
    lock        sync.Mutex  // 保护 workers 切片
    workerCache sync.Pool   // Worker 对象复用
    cond        *sync.Cond  // 无空闲 worker 时阻塞等待
    PanicHandler func()
}
```

**提交任务流程（`mspool/pool.go:GetWorker`）：**

```
Submit(task)
    └─ GetWorker()
          ├─ 有空闲 worker？→ 从队列尾部取出（O(1)）
          ├─ running < cap？→ 从 workerCache 取或新建 Worker，启动 goroutine
          └─ 否则 → cond.Wait() 阻塞，等其他 worker 完成后 Signal 唤醒
```

**Worker 运行（`mspool/worker.go`）：**

```go
func (w *Worker) running() {
    defer func() {
        if err := recover(); err != nil { /* 处理 panic */ }
        w.pool.decrRunning()
        w.pool.workerCache.Put(w)   // Worker 放回对象池复用
        w.pool.cond.Signal()         // 通知等待中的 Submit
    }()
    for f := range w.task {         // 持续从 channel 接收任务
        if f == nil { return }       // nil 是退出信号
        f()
        w.pool.PutWorker(w)          // 任务完成，将自己放回空闲队列
    }
}
```

**过期 Worker 清理（`mspool/pool.go:expireWorker`）：**

定时器每隔 expire 时间扫描空闲 worker，把闲置超时的 worker 发送 nil（退出信号）并从队列移除，防止内存泄漏。

---

### 面试问题与回答

**Q14：协程池的核心难点是什么？**

> 有三个难点：
>
> **1. 线程安全**：`workers` 切片和 `running` 计数器会被多个 goroutine 并发读写。`running` 用 `atomic.AddInt32` 原子操作，避免锁；`workers` 用 `sync.Mutex` 保护。
>
> **2. 等待唤醒机制**：当 running 达到上限，新任务必须等待。用 `sync.Cond` 实现：`cond.Wait()` 阻塞当前 goroutine 并释放锁；worker 完成任务放回队列时调 `cond.Signal()` 唤醒一个等待者。这比忙等（for 循环轮询）节省 CPU。
>
> **3. panic 安全**：任务函数可能 panic，必须在 worker 的 defer 里 recover，防止单个任务的 panic 杀死整个 worker goroutine，否则 running 计数永远不减，池最终全部耗尽。

**Q15：为什么 Worker 要用 sync.Pool 缓存而不是直接 new？**

> Worker 对象本身（包含 `task chan func()` 通道）在不同请求间可以复用。用 `sync.Pool` 缓存 Worker，Worker 退出时不是真的销毁，而是放回 Pool；下次需要新 Worker 时优先从 Pool 取，减少 `make(chan func())` 和 GC 的开销。本质上是**对象池套协程池**的双层复用设计。

**Q16：和 Go 原生 goroutine 相比，协程池有什么代价？**

> 代价主要有两点：
> 1. **延迟**：任务提交到 channel，再由 worker 从 channel 取出执行，比直接 `go func()` 多了一次 channel 通信的开销。
> 2. **复杂度**：要维护 worker 生命周期、过期清理、panic 恢复，代码复杂。
>
> 适用场景：任务量非常大（>10万/秒）且 goroutine 创建是瓶颈时才考虑。普通业务场景下 Go 的 goroutine 本来就很轻（2KB 初始栈），不用刻意池化。

---

## 8. 日志系统

### 核心实现

**多输出目标（`mslog/log.go:LoggerWriter`）：**

```go
type LoggerWriter struct {
    Level LoggerLevel   // 这个 writer 只接收哪个级别的日志
    out   io.Writer     // 输出目标（stdout / 文件）
}
```

一个 Logger 可以有多个 writer：

```go
// SetLogPath 会同时创建 4 个文件 writer
l.Outs = append(l.Outs, &LoggerWriter{Level: -1,         out: FileWriter("all.mslog")})
l.Outs = append(l.Outs, &LoggerWriter{Level: LevelDebug, out: FileWriter("debug.mslog")})
l.Outs = append(l.Outs, &LoggerWriter{Level: LevelInfo,  out: FileWriter("info.mslog")})
l.Outs = append(l.Outs, &LoggerWriter{Level: LevelError, out: FileWriter("error.mslog")})
```

**日志写入判断（`mslog/log.go:Print`）：**

```go
func (l *Logger) Print(level LoggerLevel, msg any) {
    if level < l.Level { return }  // 低于全局级别直接丢弃
    for _, out := range l.Outs {
        // Level == -1 表示 all，接收所有级别
        if level == out.Level || out.Level == -1 {
            fmt.Fprint(out.out, l.Formatter.Format(params))
            l.CheckFileSize(out)   // 每次写后检查文件大小
        }
    }
}
```

**日志文件自动轮转（`mslog/log.go:CheckFileSize`）：**

```go
func (l *Logger) CheckFileSize(w *LoggerWriter) {
    file, ok := w.out.(*os.File)
    if !ok { return }
    stat, _ := file.Stat()
    if stat.Size() >= l.LogFilesize {   // 超过阈值（默认 100MB）
        // 新文件名：all.1700000000.mslog
        writer := FileWriter(path.Join(l.LogPath, newFileName))
        w.out = writer   // 切换到新文件，原文件自动关闭（GC 时）
    }
}
```

**两种格式（`mslog/format.text.go` + `mslog/format.json.go`）：**
- `TextFormatter`：输出带颜色的人类可读格式（终端用）
- `JsonFormatter`：输出 JSON 格式（日志平台采集用）

---

### 面试问题与回答

**Q17：日志系统为什么要区分多个文件（all/debug/info/error）？**

> 实际生产中：
> - `all.mslog`：保留完整日志，排查问题时的最终依据
> - `error.mslog`：只有错误日志，用于监控告警，文件小、扫描快
> - `info.mslog`：业务流程日志，用于审计
> - `debug.mslog`：调试日志，生产环境通常不开
>
> 按级别分文件可以让运维监控只盯 error 文件，业务分析只看 info 文件，大大降低处理成本。

**Q18：日志文件轮转是怎么实现的？**

> 每次写日志后都调 `CheckFileSize` 检查当前文件大小。如果超过阈值（默认 100MB），就创建一个新文件（文件名带时间戳），把 `w.out` 替换成新的 `*os.File`。
>
> 这是**基于大小的轮转**。生产系统还常见**基于时间的轮转**（每天一个文件）。更完善的做法是接入 `lumberjack` 这类成熟的轮转库（go.mod 里有引用），支持大小+时间双维度、保留N份历史等功能。

---

## 9. JWT 认证中间件

### 核心实现

**JwtHandler 结构（`token/token.go`）：**

```go
type JwtHandler struct {
    Alg            string           // 算法：HS256/RS256 等
    Timeout        time.Duration    // access token 有效期
    RefreshTimeout time.Duration    // refresh token 有效期
    Key            []byte           // HMAC 密钥
    PrivateKey     string           // RSA 私钥（非对称算法用）
    SendCookie     bool             // 是否通过 Cookie 传递 token
    Authenticator  func(ctx *Context) (map[string]any, error)  // 用户验证函数
    AuthHandler    func(ctx *Context, err error)               // 认证失败回调
}
```

**登录流程（`LoginHandler`）：**

```
1. 调用 Authenticator(ctx) 验证用户名密码，返回 claims 数据
2. 用 jwt.New(signingMethod) 创建 token
3. 设置 claims（用户数据 + exp 过期时间 + iat 签发时间）
4. 用密钥签名得到 tokenStr
5. 调用 refreshToken() 生成 refresh token（过期时间更长，独立 claims 副本）
6. 可选：把 token 写入 Cookie
7. 返回 {Token, RefreshToken}
```

**鉴权拦截（`AuthInterceptor`）：**

```go
func (j *JwtHandler) AuthInterceptor(next HandlerFunc) HandlerFunc {
    return func(ctx *Context) {
        token := ctx.R.Header.Get(j.Header)  // 从 Authorization header 取
        if token == "" {
            if j.SendCookie {
                token, _ = ctx.GetCookie(j.CookieName)  // 降级从 Cookie 取
            } else {
                // 返回 401
                return
            }
        }
        t, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
            return j.Key, nil   // 提供密钥
        })
        if err != nil {
            // 返回 401
            return
        }
        ctx.Set("claims", t.Claims.(jwt.MapClaims))  // 存入 Context
        next(ctx)
    }
}
```

---

### 面试问题与回答

**Q19：JWT 的结构是什么？怎么验证签名？**

> JWT 由三部分组成，用 `.` 连接：
> - **Header**：`{"alg":"HS256","typ":"JWT"}` base64 编码，指定算法
> - **Payload**：`{"userId":1,"exp":1700000000}` base64 编码，存 claims
> - **Signature**：`HMACSHA256(base64(header) + "." + base64(payload), secret)`
>
> 验证时：服务端用相同的密钥和算法重新计算 signature，和 token 里的 signature 对比，一致则合法。由于不知道密钥无法伪造 signature，所以 payload 是防篡改的（但不加密，base64 可以直接解码读取）。

**Q20：access token 和 refresh token 的区别是什么？**

> - **access token**：有效期短（如 1 小时），用于访问 API。频繁传输但过期快，即使泄露危害有限。
> - **refresh token**：有效期长（如 30 天），只在续期时使用，不随每次请求传输。access token 过期后，用 refresh token 换取新的 access token，用户不用重新登录。
>
> 在代码里，refresh token 是 access token claims 的副本，但 `exp` 使用 `RefreshTimeout`（更长）。`refreshToken()` 方法复制一份新的 claims map 避免修改原 token。

**Q21：JWT 有什么缺点？如何解决？**

> 最大缺点是**无法主动失效**：token 签发后只要没过期就一直有效，服务端无法单独撤销某个 token（比如用户改密码、踢下线）。
>
> 解决方案：
> 1. **token 黑名单**：把需要失效的 token 存 Redis，每次请求时检查是否在黑名单。
> 2. **缩短有效期 + refresh 机制**：access token 只有 15 分钟，结合 refresh token 续期，泄露影响窗口短。
> 3. **在 claims 里加版本号**：用户改密码时更新版本号，验证时 token 里的版本对不上就拒绝。

---

## 10. RPC 通信

### 核心实现

#### TCP RPC 协议（`rpc/tcp/tcp.go`）

自定义二进制协议，Header 固定 17 字节：

```
| 1B magic | 1B version | 4B fullLength | 1B msgType | 1B compress | 1B serialize | 8B requestId |
```

```go
// 发送时
headers[0] = mn          // magic number：0x1d，防止粘包时误判
headers[1] = version     // 协议版本
binary.BigEndian.PutUint32(headers[2:6], uint32(fullLen))  // 大端序，全包长度
headers[6] = byte(msgResponse)
headers[7] = byte(rsp.CompressType)   // Gzip 压缩
headers[8] = byte(rsp.SerializeType)  // Gob 序列化
binary.BigEndian.PutUint64(headers[9:], uint64(rsp.RequestId))
```

**服务端反射调用（`rpc/tcp/tcp.go:readHandle`）：**

```go
// 根据 ServiceName 找到注册的服务对象
service := s.serviceMap[req.ServiceName]
v := reflect.ValueOf(service)
// 通过方法名找到方法
reflectMethod := v.MethodByName(req.MethodName)
// 构造参数
args := make([]reflect.Value, len(req.Args))
for i := range req.Args {
    args[i] = reflect.ValueOf(req.Args[i])
}
// 反射调用
result := reflectMethod.Call(args)
```

#### HTTP RPC（`rpc/http.go`）

通过 struct tag 映射方法到 HTTP 接口：

```go
type GoodsService struct {
    // msrpc tag 格式：METHOD,/path
    GetGoods func(args map[string]any) ([]byte, error) `msrpc:"GET,/goods/list"`
}
```

`client.Do("GoodsService", "GetGoods")` 时，通过反射读取 tag，给 `GetGoods` 字段赋值为实际发 HTTP 请求的函数。

---

### 面试问题与回答

**Q22：为什么 TCP RPC 协议要有 magic number？**

> TCP 是流式协议，没有天然的消息边界。如果连续发两个包，接收方可能一次读到 1.5 个包的数据（粘包），或半个包的数据（拆包）。
>
> magic number（0x1d）是协议标识符，放在每个消息的开头。接收方先读 17 字节 header，校验第 0 字节是否是 0x1d，来确认这是一个合法的消息起始位置。再从 header 里取 `fullLength`，知道后面要读多少字节的 body，用 `io.ReadFull` 精确读取，从而解决粘包/拆包问题。

**Q23：反射调用和直接调用相比有什么优缺点？**

> 优点：**通用性**。服务端不需要提前知道有哪些服务方法，只需注册 `interface{}`，运行时根据请求里的方法名动态查找并调用，一套代码支持所有服务。
>
> 缺点：
> 1. **性能**：反射调用比直接调用慢约 5-10 倍。gRPC 通过 protobuf 代码生成避免了反射，性能更高。
> 2. **类型安全**：反射调用在编译期无法检查方法名和参数类型是否正确，错误只在运行时才能发现。
> 3. **参数类型问题**：Gob 序列化/反序列化时，参数类型信息可能丢失（如 `int` 变成 `float64`），需要额外处理。

---

## 11. 服务注册与发现

### 核心实现

定义统一接口（`register/register.go`），支持多种注册中心：

```go
type MsRegister interface {
    CreateClient(option MsRegisterOption) error
    RegisterService(serviceName string, host string, port int) error
    GetService(serviceName string) (string, error)
    Close() error
}
```

**Etcd 实现（`register/etcd.go`）：**

```go
// 注册：在 etcd 中写入 key=serviceName, value=host:port
func EtcdRegisterService(option Option) error {
    cli, _ := client3.New(...)
    defer cli.Close()
    ctx, cancel := context.WithTimeout(context.Background(), time.Second)
    defer cancel()
    _, err = cli.Put(ctx, option.ServiceName, fmt.Sprintf("%s:%d", option.Host, option.Port))
    return err
}

// 发现：从 etcd 查询 key 对应的地址
func GetEtcdValue(option Option) (string, error) {
    v, err := cli.Get(ctx, option.ServiceName)
    if err != nil { return "", err }
    if len(v.Kvs) == 0 { return "", errors.New("key not found") }
    return string(v.Kvs[0].Value), nil
}
```

**网关集成（`engine/engine.go`）：**

```go
// 请求到来时，如果命中网关路由配置
if e.register != nil {
    // 从注册中心查询服务地址
    serviceName, err := e.register.GetService(gwConfig.ServiceName)
    rawURL = fmt.Sprintf("http://%s", serviceName)
}
// 反向代理转发
proxy := httputil.ReverseProxy{Director: director}
proxy.ServeHTTP(writer, request)
```

---

### 面试问题与回答

**Q24：服务注册发现解决什么问题？**

> 微服务架构中，服务实例的 IP 和端口是动态变化的（扩缩容、故障重启），不能硬编码在调用方配置里。
>
> 注册中心解决方案：
> 1. **服务注册**：服务启动时把自己的 `host:port` 写入注册中心（如 Etcd）
> 2. **服务发现**：调用方从注册中心查询目标服务的地址，不需要知道对方 IP
> 3. **健康检查**：注册中心定期检查服务存活，故障节点自动下线
>
> 这样实现了服务的**位置透明**，上下游服务解耦。

**Q25：Etcd 和 Nacos 有什么区别？为什么两个都支持？**

> - **Etcd**：基于 Raft 协议的强一致性 KV 存储，CP 系统，适合对一致性要求高的配置管理和服务注册。Go 生态原生。
> - **Nacos**：阿里开源，AP 系统（优先可用性），内置健康检查、流量管理、配置中心等功能，Java 生态广泛使用。
>
> 支持两种是为了**适配不同技术栈的团队**。通过定义统一的 `MsRegister` 接口，调用方代码不依赖具体实现，切换注册中心只需换一个 impl，体现了**依赖倒置原则**。

---

## 12. 综合性问题

**Q26：这个项目最难的部分是什么？**

> 有两个地方比较有挑战：
>
> **一是协程池的并发安全**。`GetWorker` 方法需要在高并发下正确判断是取空闲 worker 还是新建 worker 还是等待，多个 goroutine 同时操作 workers 切片，需要仔细设计锁的粒度和 cond 的使用时机，稍有不慎就会死锁或数据竞争。
>
> **二是 ORM 的反射处理**。Go 的反射 API 比较繁琐，处理指针、嵌套结构体、类型转换都有坑。比如 `Select` 时把数据库返回的 `[]byte` 值映射回结构体的 `string` 字段，需要用 `reflect.Value.Convert` 做类型转换，而且要处理 null 值、类型不兼容等边界情况。

**Q27：如果让你优化这个框架，你会从哪里入手？**

> 按优先级：
>
> 1. **路由**：当前路由级中间件是逆序包裹的，和引擎级行为不一致，容易混淆，需要统一。另外 `engine/router.go` 只有 `Get` 和 `Any`，应补全 POST/PUT/DELETE 等方法。
>
> 2. **ORM**：
>    - 缓存 reflect 结果，避免每次都重新解析结构体
>    - 支持更多查询操作（IN、LIKE、ORDER BY、LIMIT）
>    - 支持关联查询（一对多、多对多）
>
> 3. **TCP RPC**：
>    - 当前参数类型在 Gob 序列化后可能丢失（`int` 变 `float64`），需要在序列化前保留类型信息
>    - 缺少连接复用（连接池），每次请求都新建 TCP 连接，开销大
>
> 4. **Context 池**：当前只重置了部分字段，可以写一个专门的 `reset()` 方法统一处理，更安全。

**Q28：这个项目中有哪些设计模式？**

> - **中间件（装饰器模式）**：`MiddlewareFunc` 接收 handler 返回新 handler，逐层包裹，是经典装饰器。
> - **对象池（池化模式）**：`sync.Pool` 复用 Context，`mspool` 复用 goroutine 和 Worker 对象。
> - **策略模式**：日志系统的 `Formatter` 接口（`TextFormatter`/`JsonFormatter`），以及 `Serializer` 接口（`GobSerializer`），运行时可替换实现。
> - **模板方法模式**：`render.Render` 接口定义 `WriteContentType + Render` 两步模板，JSON/XML/HTML 分别实现细节。
> - **依赖倒置**：`register.MsRegister` 接口让上层（engine）不依赖 Etcd/Nacos 具体实现。
> - **责任链模式**：中间件链，每个中间件决定是否调用 `next`，继续传递或终止请求。

**Q29：项目中遇到过哪些 bug？怎么发现和修复的？**

> 遇到过几个有代表性的：
>
> **1. orm/Delete 全表删除**（严重）：`Delete` 方法构建了带 WHERE 条件的 SQL 字符串存在 `sb`，但 `Prepare` 传入的是没有 WHERE 的原始 `query` 变量。通过 code review 发现，修复是把 `Prepare(query)` 改为 `Prepare(sb.String())`。
>
> **2. Context 池重用数据泄漏**：`ServeHTTP` 把 Context 放回 pool 前没有清零 `Keys` 字段，导致上一个请求的认证信息（如 userId）可能传给下一个请求。通过分析 pool 复用机制发现，在归还前加了 `ctx.Keys = nil` 等重置操作。
>
> **3. token 验证 nil panic**：JWT 验证失败时 `err != nil`，应该 return 但漏掉了，代码继续执行 `t.Claims.(jwt.MapClaims)`，此时 `t` 为 nil 直接 panic。通过 code review 发现，补充了 `return` 语句。
>
> 这些 bug 让我意识到：**错误处理后的 return 是 Go 中最容易遗漏的编码规范**，以及**对象池复用时一定要显式重置状态**。

---

## 总结：项目亮点总结（供自我介绍使用）

> 这个项目让我深入理解了：
> 1. **前缀树路由**为什么比 map 快，以及通配符匹配的实现细节
> 2. **sync.Pool 对象复用**如何减少 GC 压力，以及复用时必须重置状态
> 3. **反射实现 ORM** 的核心原理，以及参数化查询防 SQL 注入
> 4. **协程池**如何用 sync.Cond 实现等待唤醒，避免无限创建 goroutine
> 5. **中间件洋葱模型**的函数包裹实现，以及 panic recovery 的正确姿势
> 6. **自定义 TCP 二进制协议**解决粘包问题的方法（magic number + 消息长度）
> 7. **JWT 双 token 机制**的实现和安全注意事项
