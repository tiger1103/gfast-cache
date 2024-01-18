/*
* @desc:功能测试
* @company:云南奇讯科技有限公司
* @Author: yixiaohu
* @Date:   2022/2/23 9:13
 */

package test

import (
	"context"
	"fmt"
	_ "github.com/gogf/gf/contrib/nosql/redis/v2"
	"github.com/gogf/gf/v2/database/gredis"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/tiger1103/gfast-cache/adapter"
	"github.com/tiger1103/gfast-cache/cache"
	"testing"
)

func TestBatch(t *testing.T) {
	//t.Run("testMemory", testMemory)
	//t.Run("testRedis", testRedis)
	//t.Run("testDist", testDist)
	t.Run("testDistData", testDistData)
	//t.Run("testMemoryWithTag", testMemoryWithTag)
	//t.Run("testRedisWithTag", testRedisWithTag)
	//t.Run("testDistWithTag", testDistWithTag)
}

// 缓存使用内存测试
func testMemory(t *testing.T) {
	c := cache.New("prefix")
	ctx := context.Background()
	// tag can batch Management Cache
	c.Set(ctx, "person", g.Map{"name": "zhangsan", "age": 10}, 0)
	v := c.Get(ctx, "person")
	fmt.Println(v)
	//按键删除
	c.Remove(ctx, "person")
}

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
	c.Set(ctx, "person", g.Map{"name": "zhangsan", "age": 10}, 0)
	v := c.Get(ctx, "person")
	fmt.Println(v)
	//按键删除
	c.Remove(ctx, "person")
}

// 磁盘缓存测试
func testDist(t *testing.T) {
	config := adapter.Config{
		Dir: "./distDb",
	}
	ctx := context.Background()
	adapter.SetConfig(&config)
	c := cache.NewDist("prefix")
	// tag can batch Management Cache
	c.Set(ctx, "person", g.Map{"name": "zhangsan", "age": 10}, 0)
	c.Set(ctx, "hello", "word", 0)
	v := c.Get(ctx, "person")
	fmt.Println(v)
	//按键删除
	val := c.Remove(ctx, "person")
	fmt.Println("RemoveVal is :", val)
}

func testDistData(t *testing.T) {
	type A struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	a := &A{
		Name: "张三",
		Age:  30,
	}
	config := adapter.Config{
		Dir: "./distDb",
	}
	ctx := context.Background()
	adapter.SetConfig(&config)
	c := cache.NewDist("prefix")
	//c.Set(ctx, "person", g.Map{"name": "zhangsan", "age": 10}, 0)
	//c.Set(ctx, "YXH", "我明天去", 0)
	c.Set(ctx, "aaa", a, 0)
	c.Set(ctx, "bbb", true, 0)
	//c.Set(ctx, "woooooooooooooo", "chinaaaaaaaaaaaaaaaa", 10*time.Second)
	fmt.Printf("数据库中有%d个元素\n", c.Size(ctx))
	ccc := c.Get(ctx, "aaa")

	fmt.Println("ccccccccccccccc", ccc)
	fmt.Println("bbbbbbbbbbbbbb", c.Get(ctx, "bbb"))

	var b *A
	err := ccc.Struct(&b)
	if err != nil {
		panic(err)
	}
	fmt.Println("获取到的AAAAAAAAAAAA", b.Name, b.Age)
	/*mp := c.Data(ctx)
	for k, v := range mp {
		fmt.Printf("得到的结果：%s----%s\n", gconv.String(k), gconv.String(v))
	}*/
	/*keys := c.Keys(ctx)
	for _, v := range keys {
		fmt.Println("key:", gconv.String(v))
	}
	values := c.Values(ctx)
	for _, v := range values {
		fmt.Println("value:", gconv.String(v))
	}*/
}

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
	c.Removes(ctx, []string{"person01", "person02"})
	// 使用缓存标签
	c.RemoveByTag(ctx, "tag_person") //直接就可以删除该标签下的缓存("person01","person02")
	// 甚至可以批量删除标签
	c.RemoveByTags(ctx, []string{"tag_person", "tag_family"}) // 同时删除多组标签下的数据
}

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
	c.Removes(ctx, []string{"person01", "person02"})
	// 使用缓存标签
	c.RemoveByTag(ctx, "tag_person") //直接就可以删除该标签下的缓存("person01","person02")
	// 甚至可以批量删除标签
	c.RemoveByTags(ctx, []string{"tag_person", "tag_family"}) // 同时删除多组标签下的数据
}

// 磁盘缓存标签测试
func testDistWithTag(t *testing.T) {
	config := adapter.Config{
		Dir: "./distDb",
	}
	ctx := context.Background()
	adapter.SetConfig(&config)
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
	c.Removes(ctx, []string{"person01", "person02"})
	// 使用缓存标签
	c.RemoveByTag(ctx, "tag_person") //直接就可以删除该标签下的缓存("person01","person02")
	// 甚至可以批量删除标签
	c.RemoveByTags(ctx, []string{"tag_person", "tag_family"}) // 同时删除多组标签下的数据
}
