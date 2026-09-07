# gfast-cache

基于 GoFrame v2 的缓存组件库，支持 **内存 / Redis / 磁盘（BadgerDB）** 三种后端，并提供**缓存标签（tag）** 批量管理能力。

## 特性

- 统一 API（`cache.IGCache`），切换后端无需改动业务代码
- 按 `CachePrefix` 进程级单例，同一前缀复用同一缓存实例
- 标签机制：为缓存打 tag，一键批量失效
- Redis 后端下 tag 基于 Redis Set 实现，**跨实例原子、自动去重**，多实例并发写不丢 key

## Requirements

- Go >= 1.23（GFrame v2.9.1）

## Installation

```shell
go get -u github.com/tiger1103/gfast-cache
```

> 注意：根模块没有 Go 文件，需要 import 子包（`cache` / `adapter` / `instance`）。

## 快速开始

### 内存缓存

```go
import "github.com/tiger1103/gfast-cache/cache"

c := cache.New("prefix")
ctx := context.Background()

c.Set(ctx, "person", g.Map{"name": "zhangsan", "age": 10}, 0)
v := c.Get(ctx, "person")          // *gvar.Var
c.Remove(ctx, "person")            // 按键删除
```

### Redis 缓存

```go
import (
	_ "github.com/gogf/gf/contrib/nosql/redis/v2" // 需要注册 Redis 驱动
	"github.com/gogf/gf/v2/database/gredis"
	"github.com/tiger1103/gfast-cache/cache"
)

config := gredis.Config{Address: "127.0.0.1:6379", Db: 1}
gredis.SetConfig(&config)          // 默认分组；也可 SetConfig(&config, "myGroup")

c := cache.NewRedis("prefix")      // 指定分组：cache.NewRedis("prefix", "myGroup")
c.Set(ctx, "person", g.Map{"name": "zhangsan", "age": 10}, 0)
```

### 磁盘缓存（BadgerDB）

```go
import "github.com/tiger1103/gfast-cache/adapter"
import "github.com/tiger1103/gfast-cache/cache"

// 必须先配置，否则 NewDist/New 会 panic
adapter.SetConfig(&adapter.Config{Dir: "./distDb"})

c := cache.NewDist("prefix")       // 不传前缀时 prefix 默认为空字符串
c.Set(ctx, "person", "zhangsan", 0)
v := c.Get(ctx, "person")
```

## 标签（tag）批量管理

`Set` 时传入 tag，即可按 tag 批量删除：

```go
c.Set(ctx, "person01", "p1", 0, "tag_person")
c.Set(ctx, "person02", "p2", 0, "tag_person")
c.Set(ctx, "family01", "f1", 0, "tag_family")

c.RemoveByTag(ctx, "tag_person")              // 批量删除该标签下的所有 key
c.RemoveByTags(ctx, []string{"tag_person", "tag_family"}) // 批量删除多个标签
```

- 读取与普通缓存一致，tag 只在删除时起作用。
- **tag 键前缀 `tag_` 为保留命名空间**：业务 key 请勿以 `tag_` 开头，避免与标签索引冲突。
- tag 索引本身不会自动过期：若给数据 key 设置了 TTL，数据过期后 `RemoveByTag` 会安全跳过已失效的 key（不会报错，也不会遗留孤儿数据），但索引会保留到下一次 `RemoveByTag`。
- 内存 / 磁盘后端：tag 列表在进程内以分段锁保护读改写，**进程内并发安全**。
- Redis 后端：tag 索引用 Redis Set（`SADD`/`SMEMBERS`/`SREM`），**跨实例原子**，无需额外锁。

## API 一览（cache.IGCache）

| 方法 | 说明 |
|---|---|
| `Get(ctx, key) *gvar.Var` | 读取，不存在返回 nil |
| `Set(ctx, key, value, duration, tag ...string)` | 写入；`duration<=0` 永不过期；可带 0/1 个 tag |
| `SetIfNotExist(ctx, key, value, duration, tag string) bool` | 不存在才写入 |
| `GetOrSet / GetOrSetFunc / GetOrSetFuncLock(ctx, key, ..., duration, tag string)` | 不存在则计算并写入 |
| `Remove(ctx, key) *gvar.Var` | 按键删除 |
| `Removes(ctx, keys []string)` | 批量按键删除 |
| `RemoveByTag(ctx, tag)` | 按标签删除 |
| `RemoveByTags(ctx, tags []string)` | 批量按标签删除 |
| `Contains / Data / Keys / KeyStrings / Values / Size` | 查询辅助 |

> `SetIfNotExist / GetOrSet / GetOrSetFunc / GetOrSetFuncLock` 的 tag 参数为**必填字符串**（与 `Set` 的变参 tag 不同，属历史 API 约束），不使用 tag 时传空字符串 `""`。

## 注意事项

- 错误处理：`IGCache` 方法不向上返回 error，内部失败统一通过 `g.Log().Error` 记录。
- 磁盘适配器的过期语义与 gcache 契约一致：`GetExpire` 对永不过期键返回 `0`、对不存在的键返回 `-1`。
- 测试：`go test ./...`。Redis 用例在无本地 Redis 时自动跳过；CI（GitHub Actions）内置 Redis service。

## License

MIT
