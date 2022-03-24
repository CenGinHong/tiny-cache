package consistenthash

import (
	"hash/crc32"
	"sort"
	"strconv"
)

type Hash func(data []byte) uint32

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
