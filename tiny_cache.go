package main

import (
	"fmt"
	"github.com/tiny-cache/single_flight"
	"log"
	"sync"
)

type Getter interface {
	Get(key string) ([]byte, error)
}

type GetterFunc func(key string) ([]byte, error)

// Get 函数式接口
func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}

// Group 可以理解成一个命名空间
type Group struct {
	name      string
	getter    Getter
	mainCache cache
	peers     PeerPicker
	loader    *single_flight.Group
}

var (
	mu     sync.RWMutex              // 读写锁
	groups = make(map[string]*Group) // 存储了所有的命名空间
)

// NewGroup 新建一个命名空间
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
		loader:    &single_flight.Group{},
	}
	groups[name] = g
	return g
}

// GetGroup 获取某命名空间下的cache
func GetGroup(name string) *Group {
	// 只用上读锁就好
	mu.RLock()
	defer mu.RUnlock()
	g := groups[name]
	return g
}

// RegisterPeers 将实现了 PeerPicker 接口的 HTTPPool 注入到 Group 中。
func (g *Group) RegisterPeers(peers PeerPicker) {
	if g.peers != nil {
		panic("RegisterPeerPicker called more than once")
	}
	g.peers = peers
}

// Get 从Group获取值
func (g *Group) Get(key string) (ByteView, error) {
	if key == "" {
		return ByteView{}, fmt.Errorf("key is required")
	}
	// 锁已经在mainCache中上了
	if v, ok := g.mainCache.get(key); ok {
		log.Println("[tiny-cache] hit")
		return v, nil
	}
	// 没有命中缓存，就从其他节点或者数据源中获取
	return g.load(key)
}

// load 使用 PickPeer() 方法选择节点，若非本机节点，则调用 getFromPeer() 从远程获取。若是本机节点或失败，则回退到 getLocally()
func (g *Group) load(key string) (value ByteView, err error) {
	// 加上singleFlight
	viewi, err := g.loader.Do(key, func() (interface{}, error) {
		if g.peers != nil {
			if peer, ok := g.peers.PickPeer(key); ok {
				if value, err = g.getFromPeer(peer, key); err == nil {
					return value, nil
				}
				log.Println("[tiny-cache] Filed to get from peer", err)
			}
		}
		return g.getLocally(key)
	})
	if err == nil {
		return viewi.(ByteView), nil
	}
	return
}

// 实现了 PeerGetter 接口的 httpGetter 从访问远程节点，获取缓存值
func (g *Group) getFromPeer(peer PeerGetter, key string) (ByteView, error) {
	bytes, err := peer.Get(g.name, key)
	if err != nil {
		return ByteView{}, err
	}
	return ByteView{b: bytes}, nil
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
