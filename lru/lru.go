package lru

import "container/list"

type Cache struct {
	maxBytes   int64                         // cache 的最大使用内存
	nBytes     int64                         // cache已使用的内存
	linkedList *list.List                    // 双向链表
	cache      map[string]*list.Element      // 键值映射
	OnEvicted  func(key string, value Value) // 当缓存被淘汰时触发的
}

// 键值对
type entry struct {
	key   string
	value Value
}

// Value 计算值所占用的byte
type Value interface {
	Len() int
}

func New(maxBytes int64, onEvicted func(string, Value)) *Cache {
	return &Cache{
		maxBytes:   maxBytes,
		nBytes:     0,
		linkedList: list.New(),
		cache:      make(map[string]*list.Element),
		OnEvicted:  onEvicted,
	}
}

// Get 获取值
func (c *Cache) Get(key string) (value Value, ok bool) {
	if e, ok := c.cache[key]; ok {
		// 将该值移到队头
		c.linkedList.MoveToFront(e)
		kv := e.Value.(*entry)
		return kv.value, true
	}
	return value, ok
}

// RemoveOldest 移除最久没有被使用的缓存
func (c *Cache) RemoveOldest() {
	// 找到最后一个元素
	e := c.linkedList.Back()
	if e != nil {
		// 从链表中移除
		c.linkedList.Remove(e)
		kv := e.Value.(*entry)
		// 在map中删除
		delete(c.cache, kv.key)
		// 减去容量
		c.nBytes -= int64(len(kv.key)) + int64(kv.value.Len())
		// 若定义了淘汰函数则触发淘汰函数
		if c.OnEvicted != nil {
			c.OnEvicted(kv.key, kv.value)
		}
	}
}

// Add 添加键值缓存
func (c *Cache) Add(key string, value Value) {
	// 如果key本来就存在
	if e, ok := c.cache[key]; ok {
		// 移到队头
		c.linkedList.MoveToFront(e)
		kv := e.Value.(*entry)
		// 因为value可能是不一样的，需要修改容量
		c.nBytes += int64(value.Len()) - int64(kv.value.Len())
		// 修改value
		kv.value = value
	} else {
		// 如果不存在则存入map与linkedList
		e := c.linkedList.PushFront(&entry{key, value})
		c.cache[key] = e
		c.nBytes += int64(len(key)) + int64(value.Len())
	}
	// 超出容量的开始淘汰
	for c.maxBytes != 0 && c.maxBytes < c.nBytes {
		c.RemoveOldest()
	}
}

// Len 计算kv量
func (c *Cache) Len() int {
	return c.linkedList.Len()
}
