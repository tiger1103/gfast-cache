# gfast-cache
cache with memory or redis

## Requirements

```shell script
gf version >= v2.0.0
```

## Installation

```shell script
go get -u github.com/tiger1103/gfast-cache
```

## Usage

### Import
```go
import "github.com/tiger1103/gfast-cache"
```

### Memory Cache

```go
// 缓存使用内存测试
func testMemory(t *testing.T) {
    c := cache.NewMemo("prefix")
    ctx := context.Background()
    // tag can batch Management Cache by category
    c.Set(ctx, "person", g.Map{"name": "zhangsan", "age": 10}, 0, "tag01")
    v := c.Get(ctx, "person")
    fmt.Println(v)
}
```

### Change Database Cache From In-Memory To Redis
```go
// 缓存使用redis测试
func testRedis(t *testing.T) {
    config := gredis.Config{
        Address: "127.0.0.1:6379",
        Db: 1,
    }
    ctx := context.Background()
    gredis.SetConfig(&config)
    c := cache.NewRedis("prefix")
    // tag can batch Management Cache by category
    c.Set(ctx, "person", g.Map{"name": "zhangsan", "age": 10}, 0, "tag01")
    v := c.Get(ctx, "person")
    fmt.Println(v)
    c.Remove(ctx, "person")
    c.RemoveByTag(ctx, "demo01")
}
```


