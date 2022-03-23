package TinyCache

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
