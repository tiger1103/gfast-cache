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
	"github.com/gogf/gf/v2/util/gconv"
	"testing"
)

func TestBatch(t *testing.T) {
	t.Run("testRedis", testRedis)
}

// redis测试
func testRedis(t *testing.T) {
	config := gredis.Config{
		Address: "127.0.0.1:6379",
		Db:      1,
	}
	ctx := context.Background()
	gredis.SetConfig(&config)
	redis := gredis.Instance()
	defer redis.Close(ctx)
	_, err := redis.Do(ctx, "SET", "k", "v")
	if err != nil {
		panic(err)
	}
	r, err := redis.Do(ctx, "GET", "k")
	if err != nil {
		panic(err)
	}
	fmt.Println(gconv.String(r))
}
