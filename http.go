package main

import (
	"TinyCache/consistenthash"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

const (
	defaultBasePath = "/_tinycache/"
	defaultReplicas = 50
)

type HTTPPool struct {
	self        string // baseUrl
	basePath    string
	mu          sync.Mutex
	peers       *consistenthash.Map    // 一致性哈希map
	httpGetters map[string]*httpGetter // 映射远程节点与对应的httpGetter.每一个节点对应一个httpGetter
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

// Set 实例化一致性哈希算法，添加传入节点
func (p *HTTPPool) Set(peers ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	// 创建一致性哈希结构
	p.peers = consistenthash.New(defaultReplicas, nil)
	// 在一致性哈希结构中传入节点
	p.peers.Add(peers...)
	// 创建httpGetter
	p.httpGetters = make(map[string]*httpGetter, len(peers))
	// 创建httpGetter，用于向各节点请求
	for _, peer := range peers {
		p.httpGetters[peer] = &httpGetter{baseUrl: peer + p.basePath}
	}
}

// PickPeer 包装了一致性哈希的Get方法，会根据key从哈希环中找到所需要的值
func (p *HTTPPool) PickPeer(key string) (PeerGetter, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if peer := p.peers.Get(key); peer != "" && peer != p.self {
		p.Log("Pick peer %s", peer)
		return p.httpGetters[peer], true
	}
	return nil, false
}

var _ PeerPicker = (*HTTPPool)(nil)

type httpGetter struct {
	baseUrl string
}

func (g *httpGetter) Get(group string, key string) ([]byte, error) {
	// 构建url
	u := fmt.Sprintf(
		"%v%v/%v",
		g.baseUrl,
		url.QueryEscape(group),
		url.QueryEscape(key))
	// 发起请求
	res, err := http.Get(u)
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(res.Body)

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned: %v", res.Status)
	}
	// 读取返回值
	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %v", err)
	}
	return bytes, nil
}

var _ PeerGetter = (*httpGetter)(nil)
