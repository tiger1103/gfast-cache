/*
* @desc:缓存功能
* @company:云南奇讯科技有限公司
* @Author: yixiaohu
* @Date:   2022/2/22 14:15
 */

package cache

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/gogf/gf/v2/container/gvar"
	"github.com/gogf/gf/v2/database/gredis"
	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcache"
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/tiger1103/gfast-cache/adapter"
	"github.com/tiger1103/gfast-cache/instance"
)

type IGCache interface {
	Get(ctx context.Context, key string) *gvar.Var
	Set(ctx context.Context, key string, value interface{}, duration time.Duration, tag ...string)
	Remove(ctx context.Context, key string) *gvar.Var
	Removes(ctx context.Context, keys []string)
	RemoveByTag(ctx context.Context, tag string)
	RemoveByTags(ctx context.Context, tag []string)
	SetIfNotExist(ctx context.Context, key string, value interface{}, duration time.Duration, tag string) bool
	GetOrSet(ctx context.Context, key string, value interface{}, duration time.Duration, tag string) *gvar.Var
	GetOrSetFunc(ctx context.Context, key string, f gcache.Func, duration time.Duration, tag string) *gvar.Var
	GetOrSetFuncLock(ctx context.Context, key string, f gcache.Func, duration time.Duration, tag string) *gvar.Var
	Contains(ctx context.Context, key string) bool
	Data(ctx context.Context) map[interface{}]interface{}
	Keys(ctx context.Context) []interface{}
	KeyStrings(ctx context.Context) []string
	Values(ctx context.Context) []interface{}
	Size(ctx context.Context) int
}

type GfCache struct {
	CachePrefix string //缓存前缀
	cache       *gcache.Cache
	// redisClient is set when the cache is backed by redis. When non-nil, the tag
	// index is managed with Redis Set commands (SADD/SMEMBERS/SREM), which makes tag
	// updates atomic and idempotent across multiple application instances.
	redisClient *gredis.Redis
	tagLocks    [128]sync.Mutex
}

// New 使用内存缓存
func New(cachePrefix string) *GfCache {
	instanceKey := fmt.Sprintf("%s.%s", cachePrefix, "default")
	cache := instance.GetOrSetFuncLock(instanceKey, func() interface{} {
		cache := &GfCache{
			CachePrefix: cachePrefix,
			cache:       gcache.New(),
		}
		return cache
	})
	return cache.(*GfCache)
}

// NewRedis 使用redis缓存
func NewRedis(cachePrefix string, redisName ...string) *GfCache {
	instanceKey := fmt.Sprintf("%s.%s", cachePrefix, "adapterRedis")
	cache := instance.GetOrSetFuncLock(instanceKey, func() interface{} {
		redis := g.Redis(redisName...)
		if redis == nil {
			panic(fmt.Sprintf(
				`redis instance is not configured, please set config via gredis.SetConfig or check your config file (group: %v)`,
				redisName,
			))
		}
		cache := &GfCache{
			CachePrefix: cachePrefix,
			cache:       gcache.NewWithAdapter(gcache.NewAdapterRedis(redis)),
			redisClient: redis,
		}
		return cache
	})
	return cache.(*GfCache)
}

func NewDist(cachePrefix ...string) *GfCache {
	prefix := ""
	if len(cachePrefix) > 0 {
		prefix = cachePrefix[0]
	}
	instanceKey := fmt.Sprintf("%s.%s", prefix, "adapterDist")
	cache := instance.GetOrSetFuncLock(instanceKey, func() interface{} {
		cache := &GfCache{
			CachePrefix: prefix,
			cache:       gcache.NewWithAdapter(adapter.NewDist()),
		}
		return cache
	})
	return cache.(*GfCache)
}

// getTagLock returns the mutex for the given tag
func (c *GfCache) getTagLock(tag string) *sync.Mutex {
	var hash uint32
	for i := 0; i < len(tag); i++ {
		hash = hash*31 + uint32(tag[i])
	}
	return &c.tagLocks[hash%128]
}

// tagKey returns the full cache key of the tag index. The "tag_" prefix is a
// reserved namespace: business keys should not start with "tag_".
func (c *GfCache) tagKey(tag string) string {
	return c.CachePrefix + c.setTagKey(tag)
}

// 设置tag缓存的keys
func (c *GfCache) cacheTagKey(ctx context.Context, key interface{}, tag string) {
	tagKey := c.tagKey(tag)
	if tagKey == "" {
		return
	}
	// Redis 后端：tag 索引使用 Redis Set（SADD 原子且幂等），多实例并发写不会丢 key。
	if c.redisClient != nil {
		if _, err := c.redisClient.Do(ctx, "SADD", tagKey, key); err != nil {
			g.Log().Error(ctx, err)
		}
		return
	}
	tagValue := []interface{}{key}
	value, err := c.cache.Get(ctx, tagKey)
	if err != nil {
		g.Log().Error(ctx, err)
		return
	}
	if value != nil && !value.IsNil() {
		var keyValue []interface{}
		switch v := value.Val().(type) {
		case string, []byte:
			// 内存适配器直接存列表；磁盘适配器存 JSON 编码后的列表。
			js, err := gjson.DecodeToJson(v)
			if err != nil {
				g.Log().Error(ctx, err)
				return
			}
			keyValue = gconv.SliceAny(js.Interface())
		default:
			keyValue = gconv.SliceAny(value.Val())
		}
		for _, v := range keyValue {
			if !reflect.DeepEqual(key, v) {
				tagValue = append(tagValue, v)
			}
		}
	}
	if err := c.cache.Set(ctx, tagKey, tagValue, 0); err != nil {
		g.Log().Error(ctx, err)
	}
}

// 获取带标签的键名
func (c *GfCache) setTagKey(tag string) string {
	if tag != "" {
		tag = "tag_" + tag
	}
	return tag
}

// Set sets cache with <tagKey>-<value> pair, which is expired after <duration>.
// It does not expire if <duration> <= 0.
func (c *GfCache) Set(ctx context.Context, key string, value interface{}, duration time.Duration, tag ...string) {
	if len(tag) > 0 && tag[0] != "" {
		mu := c.getTagLock(tag[0])
		mu.Lock()
		c.cacheTagKey(ctx, key, tag[0])
		mu.Unlock()
	}
	err := c.cache.Set(ctx, c.CachePrefix+key, value, duration)
	if err != nil {
		g.Log().Error(ctx, err)
	}
}

// SetIfNotExist sets cache with <tagKey>-<value> pair if <tagKey> does not exist in the cache,
// which is expired after <duration>. It does not expire if <duration> <= 0.
func (c *GfCache) SetIfNotExist(ctx context.Context, key string, value interface{}, duration time.Duration, tag string) bool {
	if tag != "" {
		mu := c.getTagLock(tag)
		mu.Lock()
		c.cacheTagKey(ctx, key, tag)
		mu.Unlock()
	}
	v, _ := c.cache.SetIfNotExist(ctx, c.CachePrefix+key, value, duration)
	return v
}

// Get returns the value of <tagKey>.
// It returns nil if it does not exist or its value is nil.
func (c *GfCache) Get(ctx context.Context, key string) *gvar.Var {
	v, err := c.cache.Get(ctx, c.CachePrefix+key)
	if err != nil {
		g.Log().Error(ctx, err)
	}
	return v
}

// GetOrSet returns the value of <tagKey>,
// or sets <tagKey>-<value> pair and returns <value> if <tagKey> does not exist in the cache.
// The tagKey-value pair expires after <duration>.
//
// It does not expire if <duration> <= 0.
func (c *GfCache) GetOrSet(ctx context.Context, key string, value interface{}, duration time.Duration, tag string) *gvar.Var {
	if tag != "" {
		mu := c.getTagLock(tag)
		mu.Lock()
		c.cacheTagKey(ctx, key, tag)
		mu.Unlock()
	}
	v, _ := c.cache.GetOrSet(ctx, c.CachePrefix+key, value, duration)
	return v
}

// GetOrSetFunc returns the value of <tagKey>, or sets <tagKey> with result of function <f>
// and returns its result if <tagKey> does not exist in the cache. The tagKey-value pair expires
// after <duration>. It does not expire if <duration> <= 0.
func (c *GfCache) GetOrSetFunc(ctx context.Context, key string, f gcache.Func, duration time.Duration, tag string) *gvar.Var {
	if tag != "" {
		mu := c.getTagLock(tag)
		mu.Lock()
		c.cacheTagKey(ctx, key, tag)
		mu.Unlock()
	}
	v, _ := c.cache.GetOrSetFunc(ctx, c.CachePrefix+key, f, duration)
	return v
}

// GetOrSetFuncLock returns the value of <tagKey>, or sets <tagKey> with result of function <f>
// and returns its result if <tagKey> does not exist in the cache. The tagKey-value pair expires
// after <duration>. It does not expire if <duration> <= 0.
//
// Note that the function <f> is executed within writing mutex lock.
func (c *GfCache) GetOrSetFuncLock(ctx context.Context, key string, f gcache.Func, duration time.Duration, tag string) *gvar.Var {
	if tag != "" {
		mu := c.getTagLock(tag)
		mu.Lock()
		c.cacheTagKey(ctx, key, tag)
		mu.Unlock()
	}
	v, _ := c.cache.GetOrSetFuncLock(ctx, c.CachePrefix+key, f, duration)
	return v
}

// Contains returns true if <tagKey> exists in the cache, or else returns false.
func (c *GfCache) Contains(ctx context.Context, key string) bool {
	v, _ := c.cache.Contains(ctx, c.CachePrefix+key)
	return v
}

// Remove deletes the <tagKey> in the cache, and returns its value.
func (c *GfCache) Remove(ctx context.Context, key string) *gvar.Var {
	v, _ := c.cache.Remove(ctx, c.CachePrefix+key)
	return v
}

// Removes deletes <keys> in the cache.
func (c *GfCache) Removes(ctx context.Context, keys []string) {
	keysWithPrefix := make([]interface{}, len(keys))
	for k, v := range keys {
		keysWithPrefix[k] = c.CachePrefix + v
	}
	if _, err := c.cache.Remove(ctx, keysWithPrefix...); err != nil {
		g.Log().Error(ctx, err)
	}
}

// RemoveByTag deletes all cache entries that were registered under the given tag.
func (c *GfCache) RemoveByTag(ctx context.Context, tag string) {
	// Redis 后端：从 Set 读取成员并 SREM 删除。使用 SREM 而非 DEL，可保留
	// SMEMBERS 读取之后被其他实例并发 SADD 的成员，避免产生孤儿数据。
	if c.redisClient != nil {
		tagKey := c.tagKey(tag)
		if tagKey == "" {
			return
		}
		v, err := c.redisClient.Do(ctx, "SMEMBERS", tagKey)
		if err != nil {
			g.Log().Error(ctx, err)
			return
		}
		members := gconv.Strings(v.Val())
		if len(members) > 0 {
			c.Removes(ctx, members)
			args := append([]interface{}{tagKey}, gconv.Interfaces(members)...)
			if _, err := c.redisClient.Do(ctx, "SREM", args...); err != nil {
				g.Log().Error(ctx, err)
			}
		}
		return
	}
	mu := c.getTagLock(tag)
	mu.Lock()
	defer mu.Unlock()
	tagKey := c.setTagKey(tag)
	//删除tagKey 对应的 key和值
	keys := c.Get(ctx, tagKey)
	if keys != nil && !keys.IsNil() {
		var ks []string
		switch v := keys.Val().(type) {
		case string, []byte:
			js, err := gjson.DecodeToJson(v)
			if err != nil {
				g.Log().Error(ctx, err)
				return
			}
			ks = gconv.SliceStr(js.Interface())
		default:
			ks = gconv.SliceStr(keys.Val())
		}
		c.Removes(ctx, ks)
	}
	c.Remove(ctx, tagKey)
}

// RemoveByTags deletes <tags> in the cache.
func (c *GfCache) RemoveByTags(ctx context.Context, tag []string) {
	for _, v := range tag {
		c.RemoveByTag(ctx, v)
	}
}

// Data returns a copy of all tagKey-value pairs in the cache as map type.
func (c *GfCache) Data(ctx context.Context) map[interface{}]interface{} {
	v, _ := c.cache.Data(ctx)
	return v
}

// Keys returns all keys in the cache as slice.
func (c *GfCache) Keys(ctx context.Context) []interface{} {
	v, _ := c.cache.Keys(ctx)
	return v
}

// KeyStrings returns all keys in the cache as string slice.
func (c *GfCache) KeyStrings(ctx context.Context) []string {
	v, _ := c.cache.KeyStrings(ctx)
	return v
}

// Values returns all values in the cache as slice.
func (c *GfCache) Values(ctx context.Context) []interface{} {
	v, _ := c.cache.Values(ctx)
	return v
}

// Size returns the size of the cache.
func (c *GfCache) Size(ctx context.Context) int {
	v, _ := c.cache.Size(ctx)
	return v
}
