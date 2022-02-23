/*
* @desc:redis缓存适配器
* @company:云南奇讯科技有限公司
* @Author: yixiaohu
* @Date:   2022/2/22 17:57
 */

package adapter

import (
	"context"
	"github.com/gogf/gf/v2/container/gvar"
	"github.com/gogf/gf/v2/database/gredis"
	"github.com/gogf/gf/v2/os/gcache"
	"time"
)

// Copyright 2020 gf Author(https://github.com/gogf/gf). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

// Redis is the gcache adapter implements using Redis server.
type Redis struct {
	redis *gredis.Redis
}

// NewRedis newAdapterMemory creates and returns a new memory cache object.
func NewRedis(redis *gredis.Redis) gcache.Adapter {
	return &Redis{
		redis: redis,
	}
}

func (c Redis) Set(ctx context.Context, key interface{}, value interface{}, duration time.Duration) error {
	var err error
	if value == nil || duration < 0 {
		_, err = c.redis.Do(ctx, "DEL", key)
	} else {
		if duration == 0 {
			_, err = c.redis.Do(ctx, "SET", key, value)
		} else {
			_, err = c.redis.Do(ctx, "SETEX", key, uint64(duration.Seconds()), value)
		}
	}
	return err
}

func (c Redis) SetMap(ctx context.Context, data map[interface{}]interface{}, duration time.Duration) error {
	if len(data) == 0 {
		return nil
	}
	// DEL.
	if duration < 0 {
		var (
			index = 0
			keys  = make([]interface{}, len(data))
		)
		for k, _ := range data {
			keys[index] = k
			index += 1
		}
		_, err := c.redis.Do(ctx, "DEL", keys...)
		if err != nil {
			return err
		}
	}
	if duration == 0 {
		var (
			index     = 0
			keyValues = make([]interface{}, len(data)*2)
		)
		for k, v := range data {
			keyValues[index] = k
			keyValues[index+1] = v
			index += 2
		}
		_, err := c.redis.Do(ctx, "MSET", keyValues...)
		if err != nil {
			return err
		}
	}
	if duration > 0 {
		var err error
		for k, v := range data {
			if err = c.Set(ctx, k, v, duration); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c Redis) SetIfNotExist(ctx context.Context, key interface{}, value interface{}, duration time.Duration) (bool, error) {
	// Execute the function and retrieve the result.
	if f, ok := value.(func() (interface{}, error)); ok {
		var err error
		value, err = f()
		if value == nil {
			return false, err
		}
	}
	// DEL.
	if duration < 0 || value == nil {
		v, err := c.redis.Do(ctx, "DEL", key, value)
		if err != nil {
			return false, err
		}
		if v.Int() == 1 {
			return true, err
		} else {
			return false, err
		}
	}
	v, err := c.redis.Do(ctx, "SETNX", key, value)
	if err != nil {
		return false, err
	}
	if v.Int() > 0 && duration > 0 {
		// Set the expire.
		_, err := c.redis.Do(ctx, "EXPIRE", key, uint64(duration.Seconds()))
		if err != nil {
			return false, err
		}
		return true, err
	}
	return false, err
}

func (c Redis) SetIfNotExistFunc(ctx context.Context, key interface{}, f gcache.Func, duration time.Duration) (bool, error) {
	isContained, err := c.Contains(ctx, key)
	if err != nil {
		return false, err
	}
	if !isContained {
		value, err := f(ctx)
		if err != nil {
			return false, err
		}
		if err = c.Set(ctx, key, value, duration); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (c Redis) SetIfNotExistFuncLock(ctx context.Context, key interface{}, f gcache.Func, duration time.Duration) (ok bool, err error) {
	return c.SetIfNotExistFunc(ctx, key, f, duration)
}

func (c Redis) Get(ctx context.Context, key interface{}) (*gvar.Var, error) {
	v, err := c.redis.Do(ctx, "GET", key)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (c Redis) GetOrSet(ctx context.Context, key interface{}, value interface{}, duration time.Duration) (*gvar.Var, error) {
	v, err := c.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return v, c.Set(ctx, key, value, duration)
	} else {
		return v, nil
	}
}

func (c Redis) GetOrSetFunc(ctx context.Context, key interface{}, f gcache.Func, duration time.Duration) (*gvar.Var, error) {
	v, err := c.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if v == nil {
		value, err := f(ctx)
		if err != nil {
			return nil, err
		}
		if value == nil {
			return nil, nil
		}
		return gvar.New(value), c.Set(ctx, key, value, duration)
	} else {
		return v, nil
	}
}

func (c Redis) GetOrSetFuncLock(ctx context.Context, key interface{}, f gcache.Func, duration time.Duration) (result *gvar.Var, err error) {
	return c.GetOrSetFunc(ctx, key, f, duration)
}

func (c Redis) Contains(ctx context.Context, key interface{}) (bool, error) {
	v, err := c.redis.Do(ctx, "EXISTS", key)
	if err != nil {
		return false, err
	}
	return v.Bool(), nil
}

func (c Redis) Size(ctx context.Context) (int, error) {
	v, err := c.redis.Do(ctx, "DBSIZE")
	if err != nil {
		return 0, err
	}
	return v.Int(), nil
}

func (c Redis) Data(ctx context.Context) (map[interface{}]interface{}, error) {
	// Keys.
	v, err := c.redis.Do(ctx, "KEYS", "*")
	if err != nil {
		return nil, err
	}
	keys := v.Slice()
	// Values.
	v, err = c.redis.Do(ctx, "MGET", keys...)
	if err != nil {
		return nil, err
	}
	values := v.Slice()
	// Compose keys and values.
	data := make(map[interface{}]interface{})
	for i := 0; i < len(keys); i++ {
		data[keys[i]] = values[i]
	}
	return data, nil
}

func (c Redis) Keys(ctx context.Context) ([]interface{}, error) {
	v, err := c.redis.Do(ctx, "KEYS", "*")
	if err != nil {
		return nil, err
	}
	return v.Slice(), nil
}

func (c Redis) Values(ctx context.Context) ([]interface{}, error) {
	// Keys.
	v, err := c.redis.Do(ctx, "KEYS", "*")
	if err != nil {
		return nil, err
	}
	keys := v.Slice()
	// Values.
	v, err = c.redis.Do(ctx, "MGET", keys...)
	if err != nil {
		return nil, err
	}
	return v.Slice(), nil
}

func (c Redis) Update(ctx context.Context, key interface{}, value interface{}) (oldValue *gvar.Var, exist bool, err error) {
	var (
		v           *gvar.Var
		oldDuration time.Duration
	)
	// TTL.
	v, err = c.redis.Do(ctx, "TTL", key)
	if err != nil {
		return
	}
	oldDuration = v.Duration()
	if oldDuration == -2 {
		// It does not exist.
		return
	}
	// Check existence.
	v, err = c.redis.Do(ctx, "GET", key)
	if err != nil {
		return
	}
	oldValue = v
	// DEL.
	if value == nil {
		_, err = c.redis.Do(ctx, "DEL", key)
		if err != nil {
			return
		}
		return
	}
	// Update the value.
	if oldDuration == -1 {
		_, err = c.redis.Do(ctx, "SET", key, value)
	} else {
		oldDuration *= time.Second
		_, err = c.redis.Do(ctx, "SETEX", key, uint64(oldDuration.Seconds()), value)
	}
	return oldValue, true, err
}

func (c Redis) UpdateExpire(ctx context.Context, key interface{}, duration time.Duration) (oldDuration time.Duration, err error) {
	var (
		v *gvar.Var
	)
	// TTL.
	v, err = c.redis.Do(ctx, "TTL", key)
	if err != nil {
		return
	}
	oldDuration = v.Duration()
	if oldDuration == -2 {
		// It does not exist.
		oldDuration = -1
		return
	}
	oldDuration *= time.Second
	// DEL.
	if duration < 0 {
		_, err = c.redis.Do(ctx, "DEL", key)
		return
	}
	// Update the expire.
	if duration > 0 {
		_, err = c.redis.Do(ctx, "EXPIRE", key, uint64(duration.Seconds()))
	}
	// No expire.
	if duration == 0 {
		v, err = c.redis.Do(ctx, "GET", key)
		if err != nil {
			return
		}
		_, err = c.redis.Do(ctx, "SET", key, v.Val())
	}
	return
}

func (c Redis) GetExpire(ctx context.Context, key interface{}) (time.Duration, error) {
	v, err := c.redis.Do(ctx, "TTL", key)
	if err != nil {
		return 0, err
	}
	switch v.Int() {
	case -1:
		return 0, nil
	case -2:
		return -1, nil
	default:
		return v.Duration() * time.Second, nil
	}
}

func (c Redis) Remove(ctx context.Context, keys ...interface{}) (lastValue *gvar.Var, err error) {
	if len(keys) == 0 {
		return nil, nil
	}
	// Retrieves the last key value.
	if v, err := c.redis.Do(ctx, "GET", keys[len(keys)-1]); err != nil {
		return nil, err
	} else {
		lastValue = v
	}
	// Deletes all given keys.
	_, err = c.redis.Do(ctx, "DEL", keys...)
	return lastValue, err
}

func (c Redis) Clear(ctx context.Context) error {
	// The "FLUSHDB" may not be available.
	if _, err := c.redis.Do(ctx, "FLUSHDB"); err != nil {
		keys, err := c.Keys(ctx)
		if err != nil {
			return err
		}
		_, err = c.Remove(ctx, keys...)
		return err
	}
	return nil
}

func (c Redis) Close(ctx context.Context) error {
	return nil
}
