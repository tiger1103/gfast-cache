/*
* @desc:磁盘缓存适配器测试
* @company:云南奇讯科技有限公司
* @Author: yixiaohu<yxh669@qq.com>
 */

package test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gogf/gf/v2/container/gvar"
	"github.com/gogf/gf/v2/test/gtest"
	"github.com/tiger1103/gfast-cache/adapter"
)

// newTestDist creates an isolated Dist adapter on a unique temp dir.
// Each test must use a distinct group name because the adapter instance
// registry is process-global.
func newTestDist(t *testing.T, group string) *adapter.Dist {
	t.Helper()
	ctx := context.Background()
	adapter.SetConfig(&adapter.Config{Dir: t.TempDir()}, group)
	d := adapter.New(group)
	if d == nil {
		t.Fatal("adapter.New returned nil")
	}
	t.Cleanup(func() { _ = d.Close(ctx) })
	return d
}

// GetExpire contract: 0 = never expire, -1 = key does not exist, >0 = remaining lifetime.
func TestDist_GetExpireContract(t *testing.T) {
	d := newTestDist(t, "grp_expire")
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		_ = d.Set(ctx, "perm", "v", 0)
		e, err := d.GetExpire(ctx, "perm")
		t.Assert(err, nil)
		t.Assert(e, time.Duration(0))

		_ = d.Set(ctx, "ttl", "v", 10*time.Second)
		e, err = d.GetExpire(ctx, "ttl")
		t.Assert(err, nil)
		t.Assert(e > 0 && e <= 10*time.Second, true)

		e, err = d.GetExpire(ctx, "missing")
		t.Assert(err, nil)
		t.Assert(e, time.Duration(-1))
	})
}

// Remove must tolerate missing keys: no error, only the existing keys are deleted,
// and lastValue is the value of the last actually-deleted key.
func TestDist_RemoveToleratesMissing(t *testing.T) {
	d := newTestDist(t, "grp_remove")
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		_ = d.Set(ctx, "k1", "v1", 0)
		_ = d.Set(ctx, "k2", "v2", 0)
		last, err := d.Remove(ctx, "k1", "missing", "k2")
		t.Assert(err, nil)
		t.Assert(last.String(), "v2")
		b, _ := d.Contains(ctx, "k1")
		t.Assert(b, false)
		b, _ = d.Contains(ctx, "k2")
		t.Assert(b, false)

		_, err = d.Remove(ctx, "no_such_key")
		t.Assert(err, nil)
	})
}

// SetIfNotExistFuncLock must not self-deadlock (the previous implementation called
// Set -> mu.Lock while already holding mu.Lock, which is not reentrant).
func TestDist_SetIfNotExistFuncLockNoDeadlock(t *testing.T) {
	d := newTestDist(t, "grp_deadlock")
	ctx := context.Background()
	done := make(chan struct{})
	go func() {
		defer close(done)
		ok, err := d.SetIfNotExistFuncLock(ctx, "k", func(ctx context.Context) (interface{}, error) {
			return "v", nil
		}, 0)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			return
		}
		if !ok {
			t.Errorf("expected ok=true, got false")
		}
	}()
	select {
	case <-done:
		val, _ := d.Get(ctx, "k")
		if val == nil || val.String() != "v" {
			t.Errorf("unexpected value after set: %v", val)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("SetIfNotExistFuncLock deadlocked")
	}
}

// GetOrSetFuncLock must execute f exactly once under the write lock even under concurrency.
func TestDist_GetOrSetFuncLockConcurrent(t *testing.T) {
	d := newTestDist(t, "grp_gosfl")
	ctx := context.Background()
	var calls int32
	f := func(ctx context.Context) (interface{}, error) {
		atomic.AddInt32(&calls, 1)
		time.Sleep(50 * time.Millisecond)
		return "computed", nil
	}
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = d.GetOrSetFuncLock(ctx, "k", f, 0)
		}()
	}
	wg.Wait()
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Fatalf("expected f to run exactly once, ran %d times", n)
	}
	val, _ := d.Get(ctx, "k")
	if val == nil || val.String() != "computed" {
		t.Fatalf("unexpected value: %v", val)
	}
}

// Update contract: preserve TTL, no-op with exist=false on missing key, delete on nil value.
func TestDist_UpdateSemantics(t *testing.T) {
	d := newTestDist(t, "grp_update")
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		_ = d.Set(ctx, "k", "old", 0)
		old, exist, err := d.Update(ctx, "k", "new")
		t.Assert(err, nil)
		t.Assert(exist, true)
		t.Assert(old.String(), "old")
		// permanent TTL must be preserved after Update
		e, err := d.GetExpire(ctx, "k")
		t.Assert(err, nil)
		t.Assert(e, time.Duration(0))

		old, exist, err = d.Update(ctx, "missing", "x")
		t.Assert(err, nil)
		t.Assert(exist, false)

		_, exist, err = d.Update(ctx, "k", nil)
		t.Assert(err, nil)
		t.Assert(exist, true)
		b, _ := d.Contains(ctx, "k")
		t.Assert(b, false)
	})
}

// UpdateExpire contract: old duration for permanent key is 0, missing key returns -1.
func TestDist_UpdateExpireSemantics(t *testing.T) {
	d := newTestDist(t, "grp_update_expire")
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		_ = d.Set(ctx, "k", "v", 0)
		old, err := d.UpdateExpire(ctx, "k", 5*time.Second)
		t.Assert(err, nil)
		t.Assert(old, time.Duration(0))
		e, _ := d.GetExpire(ctx, "k")
		t.Assert(e > 0 && e <= 5*time.Second, true)

		old, err = d.UpdateExpire(ctx, "missing", 5*time.Second)
		t.Assert(err, nil)
		t.Assert(old, time.Duration(-1))
	})
}

func TestDist_SetGetRoundTrip(t *testing.T) {
	d := newTestDist(t, "grp_roundtrip")
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		type A struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}
		_ = d.Set(ctx, "str", "hello", 0)
		_ = d.Set(ctx, "int", 123, 0)
		_ = d.Set(ctx, "bool", true, 0)
		_ = d.Set(ctx, "map", map[string]interface{}{"a": 1, "b": "x"}, 0)
		_ = d.Set(ctx, "struct", &A{Name: "张三", Age: 30}, 0)

		v, err := d.Get(ctx, "str")
		t.Assert(err, nil)
		t.Assert(v.String(), "hello")
		v, err = d.Get(ctx, "int")
		t.Assert(err, nil)
		t.Assert(v.Int(), 123)
		v, err = d.Get(ctx, "bool")
		t.Assert(err, nil)
		t.Assert(v.Bool(), true)
		v, err = d.Get(ctx, "map")
		t.Assert(err, nil)
		t.Assert(v.Map()["a"], 1)
		v, err = d.Get(ctx, "struct")
		t.Assert(err, nil)
		var a A
		err = v.Struct(&a)
		t.Assert(err, nil)
		t.Assert(a.Name, "张三")
		t.Assert(a.Age, 30)
	})
}

func TestDist_KeysValuesSize(t *testing.T) {
	d := newTestDist(t, "grp_kv")
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		for i := 0; i < 10; i++ {
			k := gvar.New(i).String()
			_ = d.Set(ctx, k, k, 0)
		}
		keys, err := d.Keys(ctx)
		t.Assert(err, nil)
		t.Assert(len(keys), 10)
		values, err := d.Values(ctx)
		t.Assert(err, nil)
		t.Assert(len(values), 10)
		size, err := d.Size(ctx)
		t.Assert(err, nil)
		t.Assert(size, 10)
		data, err := d.Data(ctx)
		t.Assert(err, nil)
		t.Assert(len(data), 10)
	})
}

// Set contract: duration < 0 or nil value deletes the key.
func TestDist_SetNegativeOrNilDeletes(t *testing.T) {
	d := newTestDist(t, "grp_neg")
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		_ = d.Set(ctx, "k", "v", 0)
		err := d.Set(ctx, "k", "v2", -1*time.Second)
		t.Assert(err, nil)
		b, _ := d.Contains(ctx, "k")
		t.Assert(b, false)

		_ = d.Set(ctx, "k2", "v", 0)
		err = d.Set(ctx, "k2", nil, 0)
		t.Assert(err, nil)
		b, _ = d.Contains(ctx, "k2")
		t.Assert(b, false)
	})
}
