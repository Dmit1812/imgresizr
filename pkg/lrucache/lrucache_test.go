package lrucache

import (
	"math/rand"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCache(t *testing.T) {
	t.Run("empty cache, uses NewCacheWithOnDelete", func(t *testing.T) {
		c := NewCacheWithOnDelete(10, nil)

		_, ok := c.Get("aaa")
		require.False(t, ok)

		_, ok = c.Get("bbb")
		require.False(t, ok)
	})

	t.Run("simple", func(t *testing.T) {
		c := NewCache(5)

		wasInCache := c.Set("aaa", 100)
		require.False(t, wasInCache)

		wasInCache = c.Set("bbb", 200)
		require.False(t, wasInCache)

		val, ok := c.Get("aaa")
		require.True(t, ok)
		require.Equal(t, 100, val)

		val, ok = c.Get("bbb")
		require.True(t, ok)
		require.Equal(t, 200, val)

		wasInCache = c.Set("aaa", 300)
		require.True(t, wasInCache)

		val, ok = c.Get("aaa")
		require.True(t, ok)
		require.Equal(t, 300, val)

		val, ok = c.Get("ccc")
		require.False(t, ok)
		require.Nil(t, val)
	})

	t.Run("clear shall work and run 3 deletions", func(t *testing.T) {
		var a int32

		myOnDeleteFunc := func(key Key, value interface{}) {
			s := key
			v := value.(int)
			atomic.AddInt32(&a, 1)
			_ = s
			_ = v
		}

		c := NewCacheWithOnDelete(5, myOnDeleteFunc)

		_ = c.Set("aaa", 100)
		_ = c.Set("bbb", 200)
		_ = c.Set("ddd", 300)

		c.Clear()

		val, ok := c.Get("aaa")
		require.False(t, ok)
		require.Nil(t, val)
		require.Equal(t, int32(3), atomic.LoadInt32(&a))
	})

	t.Run("purge logic - extra element out of 3 shall be purged", func(t *testing.T) {
		c := NewCache(3)

		_ = c.Set("aaa", 100)
		_ = c.Set("bbb", 200)
		_ = c.Set("ccc", 300)
		_ = c.Set("ddd", 400)

		val, ok := c.Get("aaa")
		require.False(t, ok)
		require.Nil(t, val)

		val, ok = c.Get("ddd")
		require.True(t, ok)
		require.Equal(t, 400, val)
	})

	t.Run("purge logic - the least used element out of 3 shall be purged", func(t *testing.T) {
		c := NewCache(3)

		_ = c.Set("aaa", 100)
		_ = c.Set("bbb", 200)
		_ = c.Set("ccc", 300)
		_, _ = c.Get("aaa")
		_, _ = c.Get("aaa")
		_, _ = c.Get("bbb")
		_, _ = c.Get("bbb")

		_ = c.Set("ddd", 400)

		val, ok := c.Get("ccc")
		require.False(t, ok)
		require.Nil(t, val)
	})
}

func TestCacheMultithreading(t *testing.T) {
	var a int32

	myOnDeleteFunc := func(key Key, value interface{}) {
		s := key
		v := value.(int)
		atomic.AddInt32(&a, 1)
		_ = s
		_ = v
	}

	c := NewCacheWithOnDelete(200, myOnDeleteFunc)

	wg := &sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 20_000; i++ {
			c.Set(Key(strconv.Itoa(i)), i)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 20_000; i++ {
			c.Get(Key(strconv.Itoa(rand.Intn(1_000_000)))) //nolint:gosec
		}
	}()

	wg.Wait()

	require.Equal(t, int32(19_800), atomic.LoadInt32(&a),
		"number of deletions from cache should be 999000, but it was not")
}
