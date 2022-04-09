package common

/*
 * fakeaddr.go
 * net.Addr which uses static values
 * By J. Stuart McMurray
 * Created 20220409
 * Last Modified 20220409
 */

// FakeAddr is a net.Addr which uses static values.
type FakeAddr struct {
	Net  string
	Addr string
}

// Network returns f.Net
func (f FakeAddr) Network() string { return f.Net }

// String return f.Addr
func (f FakeAddr) String() string { return f.Addr }
