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
	"github.com/gogf/gf/v2/crypto/gmd5"
	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcache"
	"github.com/gogf/gf/v2/util/gconv"
	"reflect"
	"sync"
	"time"
)

const (
	CTypeMemo  = 1 // 内存
	CTypeRedis = 2 // redis
)

type gfCache struct {
	CType       int    //缓存类型
	CachePrefix string //缓存前缀
	cache       *gcache.Cache
	tagSetMux   *sync.Mutex
}

//设置tag缓存的keys
func (c *gfCache) cacheTagKey(ctx context.Context, key interface{}, tag interface{}) {
	tagKey := c.setTagKey(tag)
	if tagKey != nil {
		tagValue := []interface{}{key}
		value, _ := c.cache.Get(ctx, tagKey)
		if value != nil {
			var keyValue []interface{}
			//若是字符串
			if kStr, ok := value.Val().(string); ok {
				js, err := gjson.DecodeToJson(kStr)
				if err != nil {
					g.Log().Error(ctx, err)
					return
				}
				keyValue = gconv.SliceAny(js.Interface())
			} else {
				keyValue = gconv.SliceAny(value)
			}
			for _, v := range keyValue {
				if !reflect.DeepEqual(key, v) {
					tagValue = append(tagValue, v)
				}
			}
		}
		c.cache.Set(ctx, tagKey, tagValue, 0)
	}
}

//获取带标签的键名
func (c *gfCache) setTagKey(tag interface{}) interface{} {
	if tag != nil {
		return interface{}(fmt.Sprintf("%s_tag_%s", c.CachePrefix, gmd5.MustEncryptString(gconv.String(tag))))
	}
	return ""
}

// Set sets cache with <tagKey>-<value> pair, which is expired after <duration>.
// It does not expire if <duration> <= 0.
func (c *gfCache) Set(ctx context.Context, key interface{}, value interface{}, duration time.Duration, tag ...interface{}) {
	c.tagSetMux.Lock()
	if len(tag) > 0 {
		c.cacheTagKey(ctx, key, tag[0])
	}
	err := c.cache.Set(ctx, key, value, duration)
	if err != nil {
		g.Log().Error(ctx, err)
	}
	c.tagSetMux.Unlock()
}

// SetIfNotExist sets cache with <tagKey>-<value> pair if <tagKey> does not exist in the cache,
// which is expired after <duration>. It does not expire if <duration> <= 0.
func (c *gfCache) SetIfNotExist(ctx context.Context, key interface{}, value interface{}, duration time.Duration, tag interface{}) bool {
	c.tagSetMux.Lock()
	defer c.tagSetMux.Unlock()
	c.cacheTagKey(ctx, key, tag)
	v, _ := c.cache.SetIfNotExist(ctx, key, value, duration)
	return v
}

// Get returns the value of <tagKey>.
// It returns nil if it does not exist or its value is nil.
func (c *gfCache) Get(ctx context.Context, key interface{}) interface{} {
	v, err := c.cache.Get(ctx, key)
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
func (c *gfCache) GetOrSet(ctx context.Context, key interface{}, value interface{}, duration time.Duration, tag interface{}) interface{} {
	c.tagSetMux.Lock()
	defer c.tagSetMux.Unlock()
	c.cacheTagKey(ctx, key, tag)
	v, _ := c.cache.GetOrSet(ctx, key, value, duration)
	return v
}

// GetOrSetFunc returns the value of <tagKey>, or sets <tagKey> with result of function <f>
// and returns its result if <tagKey> does not exist in the cache. The tagKey-value pair expires
// after <duration>. It does not expire if <duration> <= 0.
func (c *gfCache) GetOrSetFunc(ctx context.Context, key interface{}, f gcache.Func, duration time.Duration, tag interface{}) interface{} {
	c.tagSetMux.Lock()
	defer c.tagSetMux.Unlock()
	c.cacheTagKey(ctx, key, tag)
	v, _ := c.cache.GetOrSetFunc(ctx, key, f, duration)
	return v
}

// GetOrSetFuncLock returns the value of <tagKey>, or sets <tagKey> with result of function <f>
// and returns its result if <tagKey> does not exist in the cache. The tagKey-value pair expires
// after <duration>. It does not expire if <duration> <= 0.
//
// Note that the function <f> is executed within writing mutex lock.
func (c *gfCache) GetOrSetFuncLock(ctx context.Context, key interface{}, f gcache.Func, duration time.Duration, tag interface{}) interface{} {
	c.tagSetMux.Lock()
	defer c.tagSetMux.Unlock()
	c.cacheTagKey(ctx, key, tag)
	v, _ := c.cache.GetOrSetFuncLock(ctx, key, f, duration)
	return v
}

// Contains returns true if <tagKey> exists in the cache, or else returns false.
func (c *gfCache) Contains(ctx context.Context, key interface{}) bool {
	v, _ := c.cache.Contains(ctx, key)
	return v
}

// Remove deletes the <tagKey> in the cache, and returns its value.
func (c *gfCache) Remove(ctx context.Context, key interface{}) interface{} {
	v, _ := c.cache.Remove(ctx, key)
	return v
}

// Removes deletes <keys> in the cache.
func (c *gfCache) Removes(ctx context.Context, keys []interface{}) {
	c.cache.Remove(ctx, keys...)
}

// RemoveByTag deletes the <tag> in the cache, and returns its value.
func (c *gfCache) RemoveByTag(ctx context.Context, tag interface{}) {
	c.tagSetMux.Lock()
	tagKey := c.setTagKey(tag)
	//删除tagKey 对应的 key和值
	keys := c.Get(ctx, tagKey)
	if keys != nil {
		//如果是字符串
		if kStr, ok := keys.(string); ok {
			js, err := gjson.DecodeToJson(kStr)
			if err != nil {
				g.Log().Error(ctx, err)
				return
			}
			ks := gconv.SliceAny(js.Interface())
			c.Removes(ctx, ks)
		} else {
			ks := gconv.SliceAny(keys)
			c.Removes(ctx, ks)
		}
	}
	c.Remove(ctx, tagKey)
	c.tagSetMux.Unlock()
}

// RemoveByTags deletes <tags> in the cache.
func (c *gfCache) RemoveByTags(ctx context.Context, tag []interface{}) {
	for _, v := range tag {
		c.RemoveByTag(ctx, v)
	}
}

// Data returns a copy of all tagKey-value pairs in the cache as map type.
func (c *gfCache) Data(ctx context.Context) map[interface{}]interface{} {
	v, _ := c.cache.Data(ctx)
	return v
}

// Keys returns all keys in the cache as slice.
func (c *gfCache) Keys(ctx context.Context) []interface{} {
	v, _ := c.cache.Keys(ctx)
	return v
}

// KeyStrings returns all keys in the cache as string slice.
func (c *gfCache) KeyStrings(ctx context.Context) []string {
	v, _ := c.cache.KeyStrings(ctx)
	return v
}

// Values returns all values in the cache as slice.
func (c *gfCache) Values(ctx context.Context) []interface{} {
	v, _ := c.cache.Values(ctx)
	return v
}

// Size returns the size of the cache.
func (c *gfCache) Size(ctx context.Context) int {
	v, _ := c.cache.Size(ctx)
	return v
}
