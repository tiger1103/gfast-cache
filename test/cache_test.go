/*
* @desc:功能测试
* @company:云南奇讯科技有限公司
* @Author: yixiaohu
* @Date:   2022/2/23 9:13
 */

package test

import (
	"context"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/nosql/redis/v2"
	"github.com/gogf/gf/v2/database/gredis"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/test/gtest"
	"github.com/tiger1103/gfast-cache/adapter"
	"github.com/tiger1103/gfast-cache/cache"
)

func TestCacheMemory_Basic(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		c := cache.New("t_mem_basic")
		ctx := context.Background()

		c.Set(ctx, "k1", "v1", 0)
		t.Assert(c.Get(ctx, "k1").String(), "v1")
		t.Assert(c.Contains(ctx, "k1"), true)
		t.Assert(c.Contains(ctx, "nope"), false)

		t.Assert(c.SetIfNotExist(ctx, "k2", "v2", 0, ""), true)
		t.Assert(c.SetIfNotExist(ctx, "k2", "v2b", 0, ""), false)
		t.Assert(c.Get(ctx, "k2").String(), "v2")

		t.Assert(c.GetOrSet(ctx, "k3", "v3", 0, "").String(), "v3")
		t.Assert(c.GetOrSet(ctx, "k3", "v3x", 0, "").String(), "v3")

		t.Assert(c.GetOrSetFunc(ctx, "k4", func(ctx context.Context) (interface{}, error) {
			return "v4", nil
		}, 0, "").String(), "v4")
		t.Assert(c.GetOrSetFuncLock(ctx, "k5", func(ctx context.Context) (interface{}, error) {
			return "v5", nil
		}, 0, "").String(), "v5")

		t.Assert(c.Size(ctx), 5)
		found := false
		for _, k := range c.KeyStrings(ctx) {
			if k == "t_mem_basick1" {
				found = true
			}
		}
		t.Assert(found, true)

		c.Set(ctx, "km", g.Map{"name": "zhangsan", "age": 10}, 0)
		t.Assert(c.Get(ctx, "km").Map()["name"], "zhangsan")

		t.Assert(c.Remove(ctx, "k1").String(), "v1")
		c.Removes(ctx, []string{"k2", "k3"})
		t.Assert(c.Contains(ctx, "k2"), false)
		t.Assert(c.Contains(ctx, "k3"), false)
	})
}

func TestCacheMemory_Tag(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		c := cache.New("t_mem_tag")
		ctx := context.Background()

		c.Set(ctx, "person01", "p1", 0, "tag_person")
		c.Set(ctx, "person02", "p2", 0, "tag_person")
		c.Set(ctx, "family01", "f1", 0, "tag_family")
		t.Assert(c.Get(ctx, "person01").String(), "p1")
		t.Assert(c.Get(ctx, "family01").String(), "f1")

		c.RemoveByTag(ctx, "tag_person")
		t.Assert(c.Contains(ctx, "person01"), false)
		t.Assert(c.Contains(ctx, "person02"), false)
		t.Assert(c.Contains(ctx, "family01"), true)

		c.Set(ctx, "person03", "p3", 0, "tag_person")
		c.Set(ctx, "family02", "f2", 0, "tag_family")
		c.RemoveByTags(ctx, []string{"tag_person", "tag_family"})
		t.Assert(c.Contains(ctx, "person03"), false)
		t.Assert(c.Contains(ctx, "family02"), false)
	})
}

// SetIfNotExist / GetOrSet / GetOrSetFunc with tag must register the key into the tag.
func TestCacheMemory_TagViaOtherApis(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		c := cache.New("t_mem_tag_api")
		ctx := context.Background()

		c.SetIfNotExist(ctx, "k1", "v1", 0, "tagA")
		c.GetOrSet(ctx, "k2", "v2", 0, "tagA")
		c.GetOrSetFunc(ctx, "k3", func(ctx context.Context) (interface{}, error) {
			return "v3", nil
		}, 0, "tagA")

		t.Assert(c.Get(ctx, "k1").String(), "v1")
		c.RemoveByTag(ctx, "tagA")
		t.Assert(c.Contains(ctx, "k1"), false)
		t.Assert(c.Contains(ctx, "k2"), false)
		t.Assert(c.Contains(ctx, "k3"), false)
	})
}

func TestCacheMemory_Expire(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		c := cache.New("t_mem_expire")
		ctx := context.Background()
		c.Set(ctx, "k", "v", 100*time.Millisecond)
		t.Assert(c.Get(ctx, "k").String(), "v")
		time.Sleep(200 * time.Millisecond)
		t.Assert(c.Contains(ctx, "k"), false)
		t.Assert(c.Get(ctx, "k") == nil, true)
	})
}

func TestCacheMemory_Singleton(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		a := cache.New("t_singleton")
		b := cache.New("t_singleton")
		c := cache.New("t_singleton_other")
		t.Assert(a == b, true)
		t.Assert(a == c, false)
	})
}

// Regression: RemoveByTag on the Dist backend must not leave orphan keys when some
// data keys already expired (previously the whole Remove batch failed on any missing key).
func TestCacheDist_Tag(t *testing.T) {
	ctx := context.Background()
	adapter.SetConfig(&adapter.Config{Dir: t.TempDir()})
	c := cache.NewDist("t_dist_tag")
	t.Cleanup(func() {
		d := adapter.New()
		_ = d.Close(ctx)
	})

	c.Set(ctx, "d1", "v1", 50*time.Millisecond, "dt")
	c.Set(ctx, "d2", "v2", 50*time.Millisecond, "dt")
	c.Set(ctx, "d3", "v3", 0, "dt")
	if v := c.Get(ctx, "d3"); v == nil || v.String() != "v3" {
		t.Fatalf("unexpected d3 value: %v", v)
	}

	time.Sleep(120 * time.Millisecond)
	if c.Contains(ctx, "d1") {
		t.Fatal("d1 should have expired")
	}

	c.RemoveByTag(ctx, "dt")
	if c.Contains(ctx, "d3") {
		t.Fatal("d3 should have been removed by RemoveByTag")
	}
	if n := c.Size(ctx); n != 0 {
		t.Fatalf("expected 0 remaining keys, got %d", n)
	}
}

// Redis tests skip gracefully when no local redis is available.
// The CI workflow starts a redis service, so this test is exercised there.
func TestCacheRedis_BasicAndTag(t *testing.T) {
	ctx := context.Background()
	const redisGroup = "gfast-cache-test"
	gredis.SetConfig(&gredis.Config{Address: "127.0.0.1:6379", Db: 1}, redisGroup)
	r := g.Redis(redisGroup)
	if r == nil {
		t.Skip("redis not configured")
	}
	if _, err := r.Do(ctx, "PING"); err != nil {
		t.Skipf("redis unavailable: %v", err)
	}

	const prefix = "t_redis"
	c := cache.NewRedis(prefix, redisGroup)
	t.Cleanup(func() {
		_, _ = r.Do(ctx, "DEL", prefix+"k1", prefix+"k2", prefix+"tag_t1")
	})

	c.Set(ctx, "k1", "v1", 0, "t1")
	c.Set(ctx, "k2", "v2", 0, "t1")
	if v := c.Get(ctx, "k1"); v == nil || v.String() != "v1" {
		t.Fatalf("unexpected k1 value: %v", v)
	}
	c.RemoveByTag(ctx, "t1")
	if c.Contains(ctx, "k1") {
		t.Fatal("k1 should have been removed by RemoveByTag")
	}
	if c.Contains(ctx, "k2") {
		t.Fatal("k2 should have been removed by RemoveByTag")
	}
}
