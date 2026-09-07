# CLAUDE.md
> 本项目说明，供 AI 编码工具使用。基于 2026-02 全量源码走读与 P0/P1/P2/P3 修复后产出。

## 项目概述

`gfast-cache` 是 **GFast 后台管理框架的缓存插件库**（纯 Go 库，无可执行入口）。基于 GoFrame v2.9.1 的 `gcache` 体系封装，提供三种缓存后端 + 缓存标签（tag）批量管理能力：

| 后端 | 构造器 | 底层 |
|---|---|---|
| 内存 | `cache.New(prefix)` | `gcache.New()` |
| Redis | `cache.NewRedis(prefix, redisName...)` | `gcache.NewAdapterRedis(g.Redis(...))` |
| 磁盘 | `cache.NewDist(prefix)` | `adapter` 包自研 Badger v4 适配器 |

- 模块：`github.com/tiger1103/gfast-cache`，Go 1.23.0 / toolchain 1.24.6
- 作者：yixiaohu（云南奇讯科技），MIT 协议；`origin` = 公司私有库 `git.qjit.cn/gfast-plugins/gfast-cache`，`github` = 公共库
- 三个子包：`cache`（封装层）、`adapter`（Badger 磁盘适配器）、`instance`（自 GoFrame 拷贝的进程级单例注册表）

## 常用命令

```shell
go build ./...        # 编译
go vet ./...          # 静态检查
go test ./...         # 全套测试（Redis 用例无本地 Redis 时自动跳过；CI 内置 Redis service）
```

CI：`.github/workflows/go.yml`（build + vet + test，含 Redis service）。

## 架构要点

- **单例机制**：三个构造器均经 `instance.GetOrSetFuncLock` 按 `CachePrefix` 做进程级单例；同一前缀的内存/Redis/Dist 实例键不同（`prefix.default` / `prefix.adapterRedis` / `distCache.default`），互不冲突；`NewDist()` 不传前缀时 prefix 默认为空串（不再 panic）。
- **Key 前缀**：`GfCache` 所有读写自动拼 `CachePrefix` 前缀（`cache/cache.go`）。
- **Tag 机制**：
  - 内存 / 磁盘：tag 列表存于 `<prefix>tag_<tag>` 键（与业务 key 同命名空间，`tag_` 为保留前缀，永不过期），进程内按 tag 哈希 128 分段锁保护读-改-写（`tagLocks`）。
  - **Redis：tag 索引用 Redis Set（`SADD`/`SMEMBERS`/`SREM`）**，跨实例原子、自动去重，无需锁；`RemoveByTag` 用 SREM 保留并发新增成员（`cache/cache.go` 的 `cacheTagKey`/`RemoveByTag` 按 `redisClient` 分支）。
  - 磁盘后端 tag 列表经 JSON 序列化存储，读回按 `string/[]byte` 分支解析。
- **磁盘适配器**（`adapter/dist.go`）：实现 `gcache.Adapter` 接口；使用前必须 `adapter.SetConfig(&adapter.Config{Dir})`，否则 `New()` panic；Badger 参数硬编码（vlog 100MB / memtable 50MB / valueThreshold 512KB）；`duration=0` 存**无 TTL 条目**（ExpiresAt=0）。
- **错误处理约定**：`IGCache` 方法不向上返回 error，内部失败统一 `g.Log().Error` 记录（gcache/gf 风格）；adapter 层按契约返回 error。

## 业务接口速查（cache.IGCache）

`Get / Set(变参 tag) / Remove / Removes([]string) / RemoveByTag / RemoveByTags / SetIfNotExist / GetOrSet(Func|FuncLock) / Contains / Data / Keys / KeyStrings / Values / Size`。

注意：`SetIfNotExist / GetOrSet / GetOrSetFunc / GetOrSetFuncLock` 的 tag 参数为**必填 string**（历史 API 约束，勿改签名破坏兼容），不用 tag 传 `""`。

新增缓存后端 = 实现一个 `gcache.Adapter`（仿照 `adapter/dist.go`），再在 `cache` 包加一个 `NewXxx` 构造器，并补充 `test/` 下的测试。

## 已修复问题（2026-02，均有回归测试）

| 级别 | 问题 | 修复位置 |
|---|---|---|
| P1 | `Dist.Get` 未命中以 ERROR 记录 `ErrKeyNotFound`（日志噪音） | adapter/dist.go `Get`：`errors.Is` 判别，miss 静默 |
| P1 | `Dist.Remove` 任一 key 缺失 → 整批事务失败（孤儿 key 链） | adapter/dist.go `Remove`：跳过缺失键 |
| P1 | `Dist.SetIfNotExistFuncLock` 自死锁（锁不可重入）；`GetOrSetFuncLock` 未加锁 | adapter/dist.go：锁内 `setLocked` 拆分，两方法真正持写锁 |
| P1 | `Dist.Keys/Values/Data/Get/Remove` 迭代器/回调切片未拷贝（数据损坏） | 全部改为 `KeyCopy`/`ValueCopy`/回调内拷贝 |
| P1 | `GetExpire/UpdateExpire` 对永久键返回负时长，违反契约（0=永久/-1=缺失） | adapter/dist.go `expireRemain` + 永久键无 TTL 存储 |
| P1 | `cache.NewDist()` 无参 panic（`cachePrefix[0]` 越界） | cache/cache.go：前缀默认空串 |
| P2 | Redis tag 读-改-写跨实例非原子（丢 key） | cache/cache.go：Redis 用 Set 命令 |
| P2 | 磁盘后端 tag 列表读回为 JSON 字节无法解析（tag 失效） | cache/cache.go：`string/[]byte` 分支解析 |
| P3 | `Removes` 吞掉错误 | cache/cache.go：记日志 |
| P3 | 测试是演示脚本无断言 | test/cache_test.go、test/dist_test.go 重写（gtest 断言） |
| P3 | README import 路径/签名失真 | README.MD 重写 |
| P3 | `.gitignore` 含 PHP 遗留物；无 CI | .gitignore 清理；.github/workflows/go.yml 新增 |

## 遗留注意事项（设计边界，非 bug）

1. **tag 键 `tag_` 前缀为保留命名空间**：业务 key 勿以 `tag_` 开头，否则与 tag 索引同键冲突。
2. tag 索引永不过期、只增不减：数据 key 过期后 `RemoveByTag` 会安全跳过（不再报错），但索引残留到下次 `RemoveByTag`，建议业务定期清理。
3. Redis 模式 `RemoveByTag` 用 SREM 而非 DEL（保留并发新增成员），可能残留空 Set 键（可忽略）。
4. `gcache.Adapter.Set` 契约（duration<0 或 nil value 删除）已在 Dist 实现；`GfCache.Set` 直接透传 duration，负值语义由底层 adapter 决定。
5. 测试依赖进程级单例注册表（`instance`）不清理：新增测试必须使用**唯一**的 cache prefix / adapter group，避免跨测试串扰；adapter 测试用 `t.TempDir()` + `t.Cleanup(Close)`。
6. 凭据安全：远程 URL 勿内嵌 token（此前已清除一个 GitHub PAT）；推送凭据走 GCM。
