# TinyCache

## LRU算法

### 缓存淘汰算法

Cache缓存的数据存储在内存中，其容量必然是有限的。若设定缓存的上限是n，当存储的数据总量超出n时必然要从中淘汰一些数据，此时就要考虑使用何种淘汰策略以平衡各应用场景的需求

#### FIFO(First In First Out)

先进先出，也就是淘汰缓存中最早添加，淘汰时最老的记录。FIFO认为最早添加的记录，其不再被使用的可能性比刚添加的可能性大。其实现就是维护一个普通的队列即可，当使用量到上限时进行淘汰队首数据。但是很多场景下，最早添加的记录也最经常被使用，但却因为在缓存中呆的太长被淘汰，导致缓存命中率低

#### LFU(Least Frequently Used)

最少使用，也就是淘汰缓存中命中频率最低的。LRU认为如果数据过去被访问多次，那么将来被访问的频率也高。

其实现就是维护一个优先队列，优先队列中元素以访问量作为排序依据。当缓存数据被命中时，修改访问量的值。

LRU受历史数据影响较大，例如某个数据历史上访问次数很高，但可能他后面再无机会被访问，但却一直不能被淘汰。

#### LRU(Least Recently Used)

最近最少使用，LRU是一种相对这种的淘汰策略。LRU认为如果该数据被访问过后，在接下来一段时间也更有可能被访问。其实现就是维护一个列表，如果给记录被访问了。当淘汰时，队首元素是最久未被访问的，淘汰该记录即可。

### LRU算法实现

#### 核心数据结构

使用map和`LinkedList`组成，map存储健和值的映射关系，其中value指向`LinkedList`的元素。

链表存储真正的value，当某数据被访问后，将该链表的元素移动到末尾。

上述的操作时间复杂度均为`O(1)`

`lru/lru.go`

```go
type Cache struct {
	maxBytes   int64                         // cache 的最大使用内存
	nBytes     int64                         // cache已使用的内存
	linkedList *list.List                    // 双向链表
	cache      map[string]*list.Element      // 键值映射
	OnEvicted  func(key string, value Value) // 当缓存被淘汰时触发的回调函数
}

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
```

 

在`entry`中把`value`中的`key`也存起来的原因是当淘汰时，能够使用`O(1)`的时间复杂度将`key`从`map`中删除，属于空间换时间的做法。

为了通用性，允许值是实现了 `Value` 接口的任意类型，该接口只包含了一个方法 `Len() int`，用于返回值所占用的内存大小。

#### Get方法

```go
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
```

- 如果键对应的链表节点存在，则将对应节点移动到队尾，并返回查找到的值。
- `c.ll.MoveToFront(e)`，即将链表中的节点 `e` 移动到队尾

#### RemoveOldest方法

```go
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
```



- `c.linkedList.Back()` 取到队首节点，从链表中删除。
- `delete(c.cache, kv.key)`，从字典中 `c.cache` 删除该节点的映射关系。
- 更新当前所用的内存 `c.nbytes`。
- 如果回调函数 `OnEvicted` 不为 nil，则调用回调函数。

#### Add方法

```go
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
```

- 如果键存在，则更新对应节点的值，并将该节点移到队尾。
- 不存在则是新增场景，首先队尾添加新节点 `&entry{key, value}`, 并字典中添加 key 和节点的映射关系。
- 更新 `c.nbytes`，如果超过了设定的最大值 `c.maxBytes`，则移除最少访问的节点。

```go
// Len 计算kv量
func (c *Cache) Len() int {
	return c.linkedList.Len()
}
```

- 获取链表中的数据量



## 单机并发缓存

### 并发读写

抽象缓存数据结构`ByteView`

```go
// ByteView 只读数据结构，用于表示缓存值
type ByteView struct {
	b []byte
}

// Len 实现Value接口
func (v ByteView) Len() int {
	return len(v.b)
}

// ByteSlice 返回拷贝值，防止缓存值被外界修改
func (v ByteView) ByteSlice() []byte {
	return cloneBytes(v.b)
}

// String 返回其字符串的值
func (v ByteView) String() string {
	return string(v.b)
}

func cloneBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}
```

### 并发特性

go官库提供的`map`本身是非线程安全的，所以需要使用互斥锁包装一层方法来实现并发读写

```go
import (
	"TinyCache/lru"
	"sync"
)

type cache struct {
	mu         sync.Mutex // 互斥锁
	lru        *lru.Cache // 实例化lru策略下的cache
	cacheBytes int64      // 缓存最大使用量
}

func (c *cache) add(key string, value ByteView) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// lazy初始化
	if c.lru == nil {
		c.lru = lru.New(c.cacheBytes, nil)
	}
	c.lru.Add(key, value)
}

func (c *cache) get(key string) (value ByteView, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lru == nil {
		return
	}
	if v, ok := c.lru.Get(key); ok {
		return v.(ByteView), ok
	}
	return
}
```

- cache对lru进行了实例化，并通过加互斥锁进一步封装了Get，Set方法

### Group结构

```
TinyCache/
    |--lru/
        |--lru.go  // lru 缓存淘汰策略
    |--byteview.go // 缓存值的抽象与封装
    |--cache.go    // 并发控制
    |--tiny_cache.go // 负责与外部交互，控制缓存存储和获取的主流程
```

#### 数据源加载器Getter

当缓存未命中时，应该从数据源（文件或者数据库中）获取数据并添加到缓存中去，在这里提供Getter接口来屏蔽不同数据源直接的差异，当缓存不存在时，通过调用这个接口来进行回调

```go
type Getter interface {
	Get(key string) ([]byte, error)
}

type GetterFunc func(key string) ([]byte, error)

// Get 函数式接口
func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}
```

- 定义接口 Getter 和 回调函数 `Get(key string)([]byte, error)`，参数是 key，返回值是 []byte。
- 定义函数类型 GetterFunc，并实现 Getter 接口的 `Get` 方法。
- 函数类型实现某一个接口，称之为接口型函数，方便使用者在调用时既能够传入函数作为参数，也能够传入实现了该接口的结构体作为参数。

#### Group定义

```go
// A Group is a cache namespace and associated data loaded spread over
type Group struct {
	name      string
	getter    Getter
	mainCache cache
}

var (
	mu     sync.RWMutex
	groups = make(map[string]*Group)
)

// NewGroup create a new instance of Group
func NewGroup(name string, cacheBytes int64, getter Getter) *Group {
	if getter == nil {
		panic("nil Getter")
	}
	mu.Lock()
	defer mu.Unlock()
	g := &Group{
		name:      name,
		getter:    getter,
		mainCache: cache{cacheBytes: cacheBytes},
	}
	groups[name] = g
	return g
}

// GetGroup returns the named group previously created with NewGroup, or
// nil if there's no such group.
func GetGroup(name string) *Group {
	mu.RLock()
	g := groups[name]
	mu.RUnlock()
	return g
}
```

`Group`可以认为是一个缓存的命名空间，每个`Group`拥有一个唯一的`name`。

- name，每个Group拥有一个唯一的name，作为标识符。
- Getter，缓存未命中是获取数据源的回调
-  mainCache，在前面实现的并发缓存

`GetGroup`只涉及读并发，所以只需要上读锁。

#### Group的Get方法

```go
// Get 从Group获取值
func (g *Group) Get(key string) (ByteView, error) {
	if key == "" {
		return ByteView{}, fmt.Errorf("key is required")
	}
	// 锁已经在mainCache中上了
	if v, ok := g.mainCache.get(key); ok {
		log.Println("[TinyCache] hit")
		return v, nil
	}
	// 没有命中缓存，就从数据源中获取
	return g.load(key)
}

// load 从数据源中读取，这里暂时只实现了locally
func (g *Group) load(key string) (value ByteView, err error) {
	return g.getLocally(key)
}

// getLocally 通过回调函数Getter从数据源中读取数据
func (g *Group) getLocally(key string) (ByteView, error) {
	bytes, err := g.getter.Get(key)
	if err != nil {
		return ByteView{}, err
	}
	value := ByteView{b: cloneBytes(bytes)}
	// 将查出来的数据写进缓存
	g.populateCache(key, value)
	return value, nil
}

// populateCache 将缓存中的数据存入mainCache中
func (g *Group) populateCache(key string, value ByteView) {
	g.mainCache.add(key, value)
}
```

Get方法实现了

- 从`mainCache`中查缓存，若存在则返回缓存值
- 若缓存不存在，调用`load`方法，`load` 调用 `getLocally`（分布式场景下会调用 `getFromPeer` 从其他节点获取），`getLocally` 调用用户回调函数 `g.getter.Get()` 获取源数据，并且将源数据添加到缓存 `mainCache` 中（通过 `populateCache` 方法）

## HTTP服务端

分布式缓存需要实现节点间通信，这里建立基于HTTP的通信机制。

### HTTPPool

HTTPPool 作为连接池将所有的节点值通信的服务

```go
import (
	"fmt"
	"log"
	"net/http"
	"strings"
)

const defaultBasePath = "/_tinycache/"

type HTTPPool struct {
	self     string // baseUrl
	basePath string
}

func NewHTTPPool(self string) *HTTPPool {
	return &HTTPPool{
		self:     self,
		basePath: defaultBasePath,
	}
}

// Log 打印日志
func (p *HTTPPool) Log(format string, v ...interface{}) {
	log.Printf("[Server %s] %s", p.self, fmt.Sprintf(format, v...))
}

// 接受http请求
func (p *HTTPPool) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// url需要含有basePath
	if !strings.HasPrefix(r.URL.Path, p.basePath) {
		panic("HTTPPool serving unexpected path: " + r.URL.Path)
	}
	p.Log("%s %s", r.Method, r.URL.Path)
	// 切分，例如http://example.com/_tinycache/hello/1，,hello作为groupName,1作为key
	parts := strings.SplitN(r.URL.Path[len(p.basePath):], "/", 2)
	if len(parts) != 2 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	groupName := parts[0]
	key := parts[1]
	// 取出缓存的命名空间
	group := GetGroup(groupName)
	if group == nil {
		http.Error(w, "no such group"+groupName, http.StatusNotFound)
		return
	}
	// 获取value
	view, err := group.Get(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// 将值写回http
	w.Header().Set("Content-Type", "application/octet-stream")
	_, _ = w.Write(view.ByteSlice())
}
```

- `HTTPPool` 持有 2 个成员变量，` self`用来记录自己的地址，包括主机名/IP 和端口。`basePath`，作为节点间通讯地址的前缀，默认是 `/_tinycache/`，则 http://example.com/_tinycache/ 开头的请求就用于节点间的访问。

### HTTPPool.ServeHTTP

```go
// 处理http请求
func (p *HTTPPool) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// url需要含有basePath
	if !strings.HasPrefix(r.URL.Path, p.basePath) {
		panic("HTTPPool serving unexpected path: " + r.URL.Path)
	}
	p.Log("%s %s", r.Method, r.URL.Path)
	// 切分，例如http://example.com/_tinycache/hello/1，,hello作为groupName,1作为key
	parts := strings.SplitN(r.URL.Path[len(p.basePath):], "/", 2)
	if len(parts) != 2 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	groupName := parts[0]
	key := parts[1]
	// 取出缓存的命名空间
	group := GetGroup(groupName)
	if group == nil {
		http.Error(w, "no such group"+groupName, http.StatusNotFound)
		return
	}
	// 获取value
	view, err := group.Get(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// 将值写回http
	w.Header().Set("Content-Type", "application/octet-stream")
	_, _ = w.Write(view.ByteSlice())
}
```

- 首先判断访问路径的前缀是否是 `basePath`。
- 约定节点间获取缓存值格式为 `/<basepath>/<groupname>/<key>`，通过 groupname 得到 group 实例，再使用 `group.Get(key)` 获取缓存数据。
- 最终使用 `w.Write()` 将缓存值作为 httpResponse 的 body 返回。

## 一致性Hash

对于分布式缓存来说，当一个节点值收到请求，如果该节点并没有存储该key值对应的数据，则需要从其他节点中去获取。一般情况下使用`hash`来将不同的key映射到不同的节点上，则不同的key都能在`O(1)`的时间复杂度里找到对应的节点。

需要处理的一个问题是：当节点数量发生变化后，所有key映射的节点都会发生变化，例如从`hash(key)%10`变成`hash(key)%9`，则相当于所有的缓存值都失效了，均需要从数据源获取数据，引起缓存雪崩。

> 缓存雪崩：缓存在同一时刻全部失效（过期或者上述的hash固定映射情况），造成瞬时DB请求量大、压力骤增，引起雪崩。常因为缓存服务器宕机，或缓存设置了相同的过期时间引起。

### 一致性hash算法

一致性hash算法将key映射到$2^{32}$的空间中，并将其首尾相连，形成一个环结构

![一致性hash环](https://s2.loli.net/2022/03/27/52vm8A4iGZuNwo3.png)

- 使用节点特征（编号或者`ip`）等计算出一个`hash`值，并放置在环上

- 计算出缓`key`的`hash`值，并放置在环上，沿同一方向（例如顺时针）找到的第一个节点就是其映射节点
- 如果某节点失效（例如图中的`node3`），则原本应该在`node3`上的`key`就会被交给`node1`，同时其他区间段的`key`并不会受到影响，能较好地解决缓存血崩的问题

###  数据倾斜

如果节点较少，则容器引起`key`的倾斜，这时引入虚拟节点的概念，一个真实的节点对应多个虚拟节点，真实和虚拟节点之间使用`map`进行映射

假设 1 个真实节点对应 3 个虚拟节点，那么 peer1 对应的虚拟节点是` peer1-1`、 `peer1-2`、 `peer1-3`（通常以添加编号的方式实现），其余节点也以相同的方式操作。

- 第一步，计算虚拟节点的 Hash 值，放置在环上。
- 第二步，计算 `key `的 `Hash `值，在环上顺时针寻找到应选取的虚拟节点，例如是` peer2-1`，那么就对应真实节点 `peer2`。

虚拟节点扩充了节点的数量，解决了节点较少的情况下数据容易倾斜的问题。

### 一致性hash实现

```go
type Map struct {
	hash     Hash           // 哈希函数
	replicas int            // 虚拟节点倍数
	keys     []int          // 哈希环
	hashMap  map[int]string // 虚拟节点和真实节点之间的映射关系
}

func New(replicas int, fn Hash) *Map {
	m := &Map{replicas: replicas,
		hash:    fn,
		hashMap: make(map[int]string),
	}
	// 指定hash函数
	if m.hash == nil {
		m.hash = crc32.ChecksumIEEE
	}
	return m
}
```

哈希环只需要使用数组实际记录下node的hash值，届时使用搜索的方法去模拟环搜索的过程

```go
// Add 添加新的节点
func (m *Map) Add(keys ...string) {
	// 添加哈希节点
	for _, key := range keys {
		// 创建replicas个虚拟节点
		for i := 0; i < m.replicas; i++ {
			// 构建虚拟节点的hash值
			hash := int(m.hash([]byte(strconv.Itoa(i) + key)))
			// 加入hash值
			m.keys = append(m.keys, hash)
			// 添加映射
			m.hashMap[hash] = key
		}
	}
	// 排序
	sort.Ints(m.keys)
}
```

- `Add` 对每一个真实节点 `key`，对应创建 `m.replicas` 个虚拟节点，这里虚拟节点的名称是：`strconv.Itoa(i) + key`，即通过添加编号的方式区分不同虚拟节点。
- 使用 `m.hash()` 计算虚拟节点的哈希值，使用 `append(m.keys, hash)` 添加到环上。
- 在 `hashMap` 中增加虚拟节点和真实节点的映射关系。
- 最后一步，环上的哈希值排序。

```go
// Get 获得距离缓存key最近的节点
func (m *Map) Get(key string) string {
	// 环内没有节点
	if len(m.keys) == 0 {
		return ""
	}
	// 将key进行hash
	hash := int(m.hash([]byte(key)))
	// 寻找离hash往上一级的节点，如果没有找到的话返回n
	idx := sort.Search(len(m.keys), func(i int) bool {
		return m.keys[i] >= hash
	})
	// idx可能是n，取余是为了成环，找不到时归回0。
	// 这里取出的是真实节点的key，也就是名字
	return m.hashMap[m.keys[idx%len(m.keys)]]
}
```

- `Get `方法首先计算 key 的哈希值。
- 然后顺时针找到第一个匹配的虚拟节点的下标 `idx`，从 `m.keys` 中获取到对应的哈希值。如果 `idx == len(m.keys)`，说明应选择 `m.keys[0]`，因为 `m.keys` 是一个环状结构，所以用取余数的方式来处理这种情况。
- 最后通过 `hashMap` 映射得到真实的节点。

