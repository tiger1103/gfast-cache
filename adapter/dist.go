/*
* @desc:磁盘缓存
* @company:云南奇讯科技有限公司
* @Author: yixiaohu<yxh669@qq.com>
* @Date:   2024/1/16 9:07
 */

package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	badger "github.com/dgraph-io/badger/v4"
	"github.com/gogf/gf/v2/container/gmap"
	"github.com/gogf/gf/v2/container/gvar"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcache"
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/tiger1103/gfast-cache/instance"
)

const (
	DefaultGroupName = "default" // Default configuration group name.
	DistCacheName    = "distCache"
)

var (
	// Configuration groups.
	localConfigMap = gmap.NewStrAnyMap(true)
)

// Config 磁盘缓存配置
type Config struct {
	Dir string
}

// SetConfig sets the global configuration for specified group.
// If `name` is not passed, it sets configuration for the default group name.
func SetConfig(config *Config, name ...string) {
	group := DefaultGroupName
	if len(name) > 0 {
		group = name[0]
	}
	localConfigMap.Set(group, config)
	g.Log().Printf(context.TODO(), `SetConfig for group "%s": %+v`, group, config)
}

// New creates or returns a Dist adapter instance of given group.
// The group configuration must be set via SetConfig before calling New,
// otherwise it panics.
func New(name ...string) *Dist {
	var (
		group  = DefaultGroupName
		config *Config
		cache  *Dist
	)
	if len(name) > 0 && name[0] != "" {
		group = name[0]
	}
	instanceKey := fmt.Sprintf("%s.%s", DistCacheName, group)
	result := instance.GetOrSetFuncLock(instanceKey, func() interface{} {
		configMap := localConfigMap.Get(group)
		if configMap != nil {
			err := gconv.Struct(configMap, &config)
			if err != nil {
				panic(fmt.Sprintf(`missing configuration for distCache:"%+v"`, err))
			}
			if config == nil {
				panic(`missing configuration for distCache:"config unusable"`)
			}
		} else {
			panic(`missing configuration for distCache:"config not set"`)
		}
		option := badger.DefaultOptions(config.Dir).
			WithValueLogFileSize(100 << 20).
			WithMemTableSize(50 << 20).
			WithValueThreshold(512 << 10)
		db, err := badger.Open(option)
		if err != nil {
			panic(fmt.Sprintf(`loading dis db wrong:"%+v"`, err))
		}
		cache = &Dist{
			config: config,
			db:     db,
		}
		return cache
	})
	if result != nil {
		return result.(*Dist)
	}
	return nil
}

// NewDist creates a Dist adapter with the default group.
func NewDist() *Dist {
	return New()
}

type Dist struct {
	config *Config
	db     *badger.DB
	mu     sync.RWMutex
}

// setLocked writes `key`-`value` pair into badger. The caller must hold d.mu (write lock).
// Per the gcache.Adapter contract, a negative duration or nil value deletes the key.
// A zero duration stores the entry without any TTL, so permanent entries have
// ExpiresAt == 0, which GetExpire/UpdateExpire rely on to return 0.
func (d *Dist) setLocked(ctx context.Context, key interface{}, value interface{}, duration time.Duration) error {
	if duration < 0 || value == nil {
		_, err := d.Remove(ctx, key)
		return err
	}
	return d.db.Update(func(txn *badger.Txn) error {
		value, err := d.convertOptionToArgs(value)
		if err != nil {
			return err
		}
		e := badger.NewEntry(gconv.Bytes(key), gconv.Bytes(value))
		if duration > 0 {
			e = e.WithTTL(duration)
		}
		return txn.SetEntry(e)
	})
}

func (d *Dist) Set(ctx context.Context, key interface{}, value interface{}, duration time.Duration) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.setLocked(ctx, key, value, duration)
}

func (d *Dist) SetMap(ctx context.Context, data map[interface{}]interface{}, duration time.Duration) error {
	for k, v := range data {
		err := d.Set(ctx, k, v, duration)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *Dist) SetIfNotExist(ctx context.Context, key interface{}, value interface{}, duration time.Duration) (ok bool, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	ok, err = d.Contains(ctx, key)
	if err != nil {
		return false, err
	}
	if ok {
		return false, nil
	}
	err = d.setLocked(ctx, key, value, duration)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (d *Dist) SetIfNotExistFunc(ctx context.Context, key interface{}, f gcache.Func, duration time.Duration) (ok bool, err error) {
	ok, err = d.Contains(ctx, key)
	if err != nil {
		return false, err
	}
	if ok {
		return false, nil
	}
	value, err := f(ctx)
	if err != nil {
		return false, err
	}
	err = d.Set(ctx, key, value, duration)
	if err != nil {
		return false, err
	}
	return true, nil
}

// SetIfNotExistFuncLock executes function `f` within the writing mutex lock for concurrent safety.
func (d *Dist) SetIfNotExistFuncLock(ctx context.Context, key interface{}, f gcache.Func, duration time.Duration) (ok bool, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	ok, err = d.Contains(ctx, key)
	if err != nil {
		return false, err
	}
	if ok {
		return false, nil
	}
	value, err := f(ctx)
	if err != nil {
		return false, err
	}
	err = d.setLocked(ctx, key, value, duration)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (d *Dist) Get(ctx context.Context, key interface{}) (value *gvar.Var, err error) {
	err = d.db.View(func(txn *badger.Txn) error {
		item, e := txn.Get(gconv.Bytes(key))
		if e != nil {
			if errors.Is(e, badger.ErrKeyNotFound) {
				// A cache miss is not an error.
				return nil
			}
			return e
		}
		return item.Value(func(val []byte) error {
			// Copy the value: the given slice is only valid within the callback.
			value = gvar.New(append([]byte(nil), val...))
			return nil
		})
	})
	return
}

func (d *Dist) GetOrSet(ctx context.Context, key interface{}, value interface{}, duration time.Duration) (result *gvar.Var, err error) {
	result, err = d.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if result != nil && !result.IsEmpty() {
		return result, nil
	}
	if err = d.Set(ctx, key, value, duration); err != nil {
		return nil, err
	}
	return gvar.New(value), nil
}

func (d *Dist) GetOrSetFunc(ctx context.Context, key interface{}, f gcache.Func, duration time.Duration) (result *gvar.Var, err error) {
	result, err = d.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if result != nil && !result.IsEmpty() {
		return result, nil
	}
	value, err := f(ctx)
	if err != nil {
		return nil, err
	}
	if err = d.Set(ctx, key, value, duration); err != nil {
		return nil, err
	}
	return gvar.New(value), nil
}

// GetOrSetFuncLock retrieves the value of `key`, or executes function `f` within the
// writing mutex lock and sets `key` with its result. This prevents cache penetration
// under concurrent access: `f` runs exactly once.
func (d *Dist) GetOrSetFuncLock(ctx context.Context, key interface{}, f gcache.Func, duration time.Duration) (result *gvar.Var, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	result, err = d.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if result != nil && !result.IsEmpty() {
		return result, nil
	}
	value, err := f(ctx)
	if err != nil {
		return nil, err
	}
	if err = d.setLocked(ctx, key, value, duration); err != nil {
		return nil, err
	}
	return gvar.New(value), nil
}

func (d *Dist) Contains(ctx context.Context, key interface{}) (b bool, err error) {
	val, err := d.Get(ctx, key)
	if err != nil {
		return false, err
	}
	return val != nil, nil
}

func (d *Dist) Size(ctx context.Context) (size int, err error) {
	// 创建一个只读事务
	err = d.db.View(func(txn *badger.Txn) error {
		// 创建一个迭代器
		iterator := txn.NewIterator(badger.DefaultIteratorOptions)
		defer iterator.Close()
		// 遍历键值对并计数
		for iterator.Rewind(); iterator.Valid(); iterator.Next() {
			size++
		}
		return nil
	})
	return
}

func (d *Dist) Data(ctx context.Context) (data map[interface{}]interface{}, err error) {
	data = make(map[interface{}]interface{})
	err = d.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			// KeyCopy returns a copy of the key, safe to use after the txn.
			k := item.KeyCopy(nil)
			val, e := item.ValueCopy(nil)
			if e != nil {
				return e
			}
			data[gconv.String(k)] = val
		}
		return nil
	})
	return
}

func (d *Dist) Keys(ctx context.Context) (keys []interface{}, err error) {
	keys = make([]interface{}, 0, 1000)
	err = d.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			// KeyCopy: item.Key() is only valid until the next iterator step.
			keys = append(keys, item.KeyCopy(nil))
		}
		return nil
	})
	return
}

func (d *Dist) Values(ctx context.Context) (values []interface{}, err error) {
	values = make([]interface{}, 0, 1000)
	err = d.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			err = item.Value(func(v []byte) error {
				// Copy the value: the given slice is only valid within the callback.
				values = append(values, append([]byte(nil), v...))
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	return
}

func (d *Dist) Update(ctx context.Context, key interface{}, value interface{}) (oldValue *gvar.Var, exist bool, err error) {
	duration, err := d.GetExpire(ctx, key)
	if err != nil {
		return nil, false, err
	}
	if duration == -1 {
		// The key does not exist or has expired: do nothing.
		return nil, false, nil
	}
	oldValue, err = d.Get(ctx, key)
	if err != nil {
		return nil, false, err
	}
	if value == nil {
		_, err = d.Remove(ctx, key)
		return oldValue, true, err
	}
	err = d.Set(ctx, key, value, duration)
	return oldValue, true, err
}

func (d *Dist) UpdateExpire(ctx context.Context, key interface{}, duration time.Duration) (oldDuration time.Duration, err error) {
	err = d.db.Update(func(txn *badger.Txn) error {
		item, err := txn.Get(gconv.Bytes(key))
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				// The key does not exist: returns -1 and does nothing.
				oldDuration = -1
				return nil
			}
			return err
		}
		oldDuration = d.expireRemain(item.ExpiresAt())
		if duration < 0 {
			// Negative duration deletes the key.
			return txn.Delete(gconv.Bytes(key))
		}
		val, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		if duration == 0 {
			// Remove the TTL: store a new entry without expiration.
			return txn.SetEntry(badger.NewEntry(gconv.Bytes(key), val))
		}
		return txn.SetEntry(badger.NewEntry(gconv.Bytes(key), val).WithTTL(duration))
	})
	return
}

// GetExpire returns the remaining lifetime of `key`.
// It returns 0 if the key never expires, and -1 if the key does not exist.
func (d *Dist) GetExpire(ctx context.Context, key interface{}) (duration time.Duration, err error) {
	err = d.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(gconv.Bytes(key))
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				duration = -1
				return nil
			}
			return err
		}
		duration = d.expireRemain(item.ExpiresAt())
		return nil
	})
	return
}

// expireRemain returns the remaining lifetime of an entry with the given unix-seconds
// expiration timestamp. It returns 0 if the entry never expires (timestamp 0) or
// has already expired.
func (d *Dist) expireRemain(expiresAt uint64) time.Duration {
	if expiresAt == 0 {
		return 0
	}
	remain := time.Until(time.Unix(int64(expiresAt), 0))
	if remain < 0 {
		return 0
	}
	return remain
}

func (d *Dist) Remove(ctx context.Context, keys ...interface{}) (lastValue *gvar.Var, err error) {
	err = d.db.Update(func(txn *badger.Txn) error {
		for _, key := range keys {
			item, err := txn.Get(gconv.Bytes(key))
			if err != nil {
				if errors.Is(err, badger.ErrKeyNotFound) {
					// Missing keys are skipped and never abort the batch.
					continue
				}
				return err
			}
			err = item.Value(func(val []byte) error {
				lastValue = gvar.New(append([]byte(nil), val...))
				return nil
			})
			if err != nil {
				return err
			}
			if err = txn.Delete(gconv.Bytes(key)); err != nil {
				return err
			}
		}
		return nil
	})
	return
}

func (d *Dist) Clear(ctx context.Context) error {
	err := d.db.DropAll()
	return err
}

func (d *Dist) Close(ctx context.Context) error {
	err := d.db.Close()
	return err
}

func (d *Dist) convertOptionToArgs(option interface{}) (result interface{}, err error) {
	if option == nil {
		return nil, nil
	}
	switch reflect.TypeOf(option).Kind() {
	case reflect.Ptr:
		// 解引用指针
		elem := reflect.ValueOf(option).Elem().Interface()
		return d.convertOptionToArgs(elem)
	case reflect.Struct:
		// 将结构体转换为 map
		result = gconv.Map(option)
		return d.convertOptionToArgs(result)
	case reflect.Bool:
		result = gconv.String(option)
	case reflect.Slice, reflect.Array, reflect.Map:
		result, err = json.Marshal(option)
	default:
		result = option
	}
	return
}
