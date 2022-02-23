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
	"github.com/gogf/gf/v2/database/gredis"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/tiger1103/gfast-cache/cache"
	"testing"
)

func TestBatch(t *testing.T) {
	//t.Run("testMemory", testMemory)
	t.Run("testRedis", testRedis)
}

// 缓存使用内存测试
func testMemory(t *testing.T) {
	c := cache.NewMemo("yxh")
	ctx := context.Background()
	c.Set(ctx, "person", g.Map{"name": "zhangsan", "age": 10}, 0, "demo01")
	v := c.Get(ctx, "person")
	fmt.Println(v)
}

// 缓存使用redis测试
func testRedis(t *testing.T) {
	config := gredis.Config{
		Address: "127.0.0.1:6379",
		Db:      1,
	}
	ctx := context.Background()
	gredis.SetConfig(&config)
	c := cache.NewRedis("prefix001")
	c.Set(ctx, "person", g.Map{"name": "zhangsan", "age": 10}, 0, "demo01")
	v := c.Get(ctx, "person")
	fmt.Println(v)
	c.Remove(ctx, "person")
	c.RemoveByTag(ctx, "demo01")
}
