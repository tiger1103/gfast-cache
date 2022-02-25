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
    c := cache.New("prefix")
    ctx := context.Background()
    // tag can batch Management Cache
    c.Set(ctx, "person", g.Map{"name": "zhangsan", "age": 10}, 0, "tag01")
    v := c.Get(ctx, "person")
    fmt.Println(v)
    //按键删除
    c.Remove(ctx, "person")
    //按标签删除
    c.RemoveByTag(ctx, "demo01")
}
```

### Change Database Cache From In-Memory To Redis
```go
// 缓存使用redis测试
func testRedis(t *testing.T) {
    config := gredis.Config{
        Address: "127.0.0.1:6379",
        Db:      1,
    }
    ctx := context.Background()
    gredis.SetConfig(&config)
    c := cache.NewRedis("prefix")
    // tag can batch Management Cache
    c.Set(ctx, "person", g.Map{"name": "zhangsan", "age": 10}, 0, "tag01")
    v := c.Get(ctx, "person")
    fmt.Println(v)
    //按键删除
    c.Remove(ctx, "person")
    //按标签删除
    c.RemoveByTag(ctx, "demo01")
}
```

### tag can batch Management Cache

```go
// 缓存标签使用内存测试
func testMemoryWithTag(t *testing.T) {
    c := cache.New("prefix")
    ctx := context.Background()
    // tag can batch Management Cache
    
    c.Set(ctx, "person01", g.Map{"name": "zhangsan", "age": 10}, 0, "tag_person")
    c.Set(ctx, "family01", g.Map{"address": "Cai Yun street"}, 0, "tag_family")
    c.Set(ctx, "work01", g.Map{"unit": "qixun"}, 0, "tag_work")
    
    c.Set(ctx, "person02", g.Map{"name": "zhangsan", "age": 10}, 0, "tag_person")
    c.Set(ctx, "family02", g.Map{"address": "Cai Yun street"}, 0, "tag_family")
    c.Set(ctx, "work02", g.Map{"unit": "qixun"}, 0, "tag_work")
    
    p1 := c.Get(ctx, "person01")
    p2 := c.Get(ctx, "person02")
    fmt.Println(p1, p2)
    // 缓存标签在读取缓存数据时和直接缓存读取一样，差别只在删除时可以批量删除
    // 比如要删除 person01和person02两组对应的缓存
    // 不使用tag时
    c.Remove(ctx, "person01")
    c.Remove(ctx, "person02")
    //或
    c.Removes(ctx, []interface{}{"person01", "person02"})
    // 使用缓存标签
    c.RemoveByTag(ctx, "tag_person") //直接就可以删除该标签下的缓存("person01","person02")
    // 甚至可以批量删除标签
    c.RemoveByTags(ctx, []interface{}{"tag_person", "tag_family"}) // 同时删除多组标签下的数据
}
```

```go

// 缓存使用redis测试
func testRedisWithTag(t *testing.T) {
    config := gredis.Config{
        Address: "127.0.0.1:6379",
        Db:      1,
    }
    ctx := context.Background()
    gredis.SetConfig(&config)
    c := cache.NewRedis("prefix")
    // tag can batch Management Cache
    
    c.Set(ctx, "person01", g.Map{"name": "zhangsan", "age": 10}, 0, "tag_person")
    c.Set(ctx, "family01", g.Map{"address": "Cai Yun street"}, 0, "tag_family")
    c.Set(ctx, "work01", g.Map{"unit": "qixun"}, 0, "tag_work")
    
    c.Set(ctx, "person02", g.Map{"name": "zhangsan", "age": 10}, 0, "tag_person")
    c.Set(ctx, "family02", g.Map{"address": "Cai Yun street"}, 0, "tag_family")
    c.Set(ctx, "work02", g.Map{"unit": "qixun"}, 0, "tag_work")
    
    p1 := c.Get(ctx, "person01")
    p2 := c.Get(ctx, "person02")
    fmt.Println(p1, p2)
    // 缓存标签在读取缓存数据时和直接缓存读取一样，差别只在删除时可以批量删除
    // 比如要删除 person01和person02两组对应的缓存
    // 不使用tag时
    c.Remove(ctx, "person01")
    c.Remove(ctx, "person02")
    //或
    c.Removes(ctx, []interface{}{"person01", "person02"})
    // 使用缓存标签
    c.RemoveByTag(ctx, "tag_person") //直接就可以删除该标签下的缓存("person01","person02")
    // 甚至可以批量删除标签
    c.RemoveByTags(ctx, []interface{}{"tag_person", "tag_family"}) // 同时删除多组标签下的数据
}
```
