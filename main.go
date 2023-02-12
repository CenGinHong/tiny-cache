package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
)

var db = map[string]string{
	"Tom":  "630",
	"Jack": "589",
	"Sam":  "567",
}

func createGroup() *Group {
	return NewGroup("scores", 2<<10, GetterFunc(
		func(key string) ([]byte, error) {
			log.Println("[SlowDB] search key", key)
			if v, ok := db[key]; ok {
				return []byte(v), nil
			}
			return nil, fmt.Errorf("%s not exist", key)
		}))
}

func startCacheServer(addr string, addrs []string, tiny *Group) {
	// 启动HTTPPool，传入地址
	peers := NewHTTPPool(addr)
	// 设置各节点监听的值
	peers.Set(addrs...)
	// 注册HTTPPool
	tiny.RegisterPeers(peers)
	log.Println("tinycache is running at", addr)
	log.Fatal(http.ListenAndServe(addr[7:], peers))
}

func startAPIServer(apiAddr string, tiny *Group) {
	http.Handle("/api", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			key := r.URL.Query().Get("key")
			view, err := tiny.Get(key)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(view.ByteSlice())

		}))
	log.Println("fontend server is running at", apiAddr)
	log.Fatal(http.ListenAndServe(apiAddr[7:], nil))
}

func main() {
	path := os.Args[1]
	config, err := LoadConfig(path)
	if err != nil {
		log.Printf("config error: %v\n", err)
		return
	}
	self := "http://" + net.JoinHostPort("localhost", config.PeerPort)
	isExist := false
	for _, v := range config.Peer {
		if v == self {
			isExist = true
			break
		}
	}

	if !isExist {
		config.Peer = append(config.Peer, self)
	}
	tiny := createGroup()
	// 主监听入口
	if config.ApiPort != "" {
		go startAPIServer("http://"+net.JoinHostPort("localhost", config.ApiPort), tiny)
	}
	startCacheServer(self, config.Peer, tiny)
}
