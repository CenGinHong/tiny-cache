package single_flight

import "sync"

type call struct {
	wg  sync.WaitGroup
	val interface{}
	err error
}

type Group struct {
	mu sync.Mutex
	m  map[string]*call
}

func (g *Group) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*call)
	}
	// 如果该key对应存在call的话
	if c, ok := g.m[key]; ok {
		// 解锁并等待
		g.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err
	}
	// 目前没有正在进行的同一个key的请求
	c := &call{}
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()
	// 执行fn执行进程
	c.val, c.err = fn()
	c.wg.Done()
	g.mu.Lock()
	// 注意这里只是删除了key，没有删除call，在if语句中获得的call仍然存在，故仍能返回值
	delete(g.m, key)
	g.mu.Unlock()
	return c.val, c.err
}
