package lrucache

import "sync"

// We will store this record in the queue - it allows to remove the item from dictionary on queue overflow.
type record struct {
	key   string
	value interface{}
}

// Note - this function should be concurrency safe
// it should be fast to return i.e. record what to delete and return - do not run long operations in this
// handler.
type OnDeleteFunc func(key string, value interface{})

type Cache interface {
	Set(key string, value interface{}) bool
	Get(key string) (interface{}, bool)
	Clear()
}

type lruCache struct {
	capacity     int
	queue        DLList
	items        map[string]*DLListItem
	mu           sync.RWMutex
	onDeleteFunc OnDeleteFunc
}

// Set puts a value for the key into the cache and moves it to the front of the queue
// reduces the size of the cache if it is over the limit
// by removing item from the bottom of the queue (the least accessed one).
func (c *lruCache) Set(key string, value interface{}) bool {
	// create record to store in the queue
	r := record{key, value}
	// is element key in the cache
	c.mu.Lock()
	if el, ok := c.items[key]; ok {
		// if yes - update the value and move to queue start
		el.Value = r
		c.queue.MoveToFront(el)
		c.mu.Unlock()
		return true
	}
	// if element not in the cache and capacity is over 0 - add the key to dictionary and add to start of queue
	if c.capacity > 0 {
		el := c.queue.PushFront(r)
		c.items[key] = el
	}

	//    in case cache size is greater then capacity - remove the last element and its key from the dictionary
	if c.capacity > 0 && c.queue.Len() > c.capacity {
		el := c.queue.Back()

		// make a copy of the element to avoid deadlock
		if c.onDeleteFunc != nil {
			ec := el.Value.(record)
			c.onDeleteFunc(ec.key, ec.value)
		}

		// remove the key from the dictionary
		delete(c.items, el.Value.(record).key)
		c.queue.Remove(el)
	}
	c.mu.Unlock()
	// return if the value was in cache
	return false
}

// Get returns the value for a given key, and moves the element with this key to the front of the queue.
func (c *lruCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	// if the key in dictionary then move this element to queue start and return its value and true
	if el, ok := c.items[key]; ok {
		c.queue.MoveToFront(el)
		c.mu.Unlock()
		return el.Value.(record).value, true
	}
	c.mu.Unlock()
	// if the key is not in dictionary return nil and false
	return nil, false
}

// Clear removes all elements from the cache by creating new pointers and counting on GC.
func (c *lruCache) Clear() {
	// We count on the fact that Go cleares data not referenced any more so we simply create new dllists for cache to work
	// without the actual clearing.
	// However in case we do run clear we have to execute onDeleteFunc for every element from the list
	c.mu.RLock()
	el := c.queue.Front()
	for el != nil {
		if c.onDeleteFunc != nil {
			c.onDeleteFunc(el.Value.(record).key, el.Value.(record).value)
		}
		el = el.Next
	}
	c.mu.RUnlock()

	c.mu.Lock()
	c.queue = NewDLList()
	c.items = make(map[string]*DLListItem, c.capacity)
	c.mu.Unlock()
}

// NewCache creates a new cache and returns it.
func NewCache(capacity int) Cache {
	if capacity < 0 {
		capacity = 0
	}
	return &lruCache{
		capacity:     capacity,
		queue:        NewDLList(),
		items:        make(map[string]*DLListItem, capacity),
		onDeleteFunc: nil,
	}
}

// NewCache creates a new cache and returns it.
func NewCacheWithOnDelete(capacity int, onDeleteFunc OnDeleteFunc) Cache {
	if capacity < 0 {
		capacity = 0
	}
	return &lruCache{
		capacity:     capacity,
		queue:        NewDLList(),
		items:        make(map[string]*DLListItem, capacity),
		onDeleteFunc: onDeleteFunc,
	}
}
