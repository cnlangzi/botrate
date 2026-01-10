package analyzer

import (
	"hash/maphash"
	"sync"
	"sync/atomic"
	"time"
)

type Config struct {
	Window        time.Duration
	PageThreshold int
	QueueCap      int
}

type Request struct {
	IP   string
	Path uint64
}

type Analyzer struct {
	cfg Config

	// Hot path: atomic blocklist with string keys
	blocklist atomic.Pointer[map[string]struct{}]

	// Cold path: event queue
	queue chan *Request

	// Worker state
	bloom   *DoubleBufferBloom
	counter *Counter

	// Close channel for cleanup
	stop chan struct{}

	// Object pool for Request reuse
	pool sync.Pool
}

func New(cfg Config) *Analyzer {
	a := &Analyzer{
		cfg:     cfg,
		queue:   make(chan *Request, cfg.QueueCap),
		bloom:   NewDoubleBufferBloom(),
		counter: NewCounter(),
		stop:    make(chan struct{}),
		pool: sync.Pool{
			New: func() interface{} {
				return &Request{}
			},
		},
	}

	bl := make(map[string]struct{})
	a.blocklist.Store(&bl)

	go a.worker()
	return a
}

func (a *Analyzer) Record(ip, path string) {
	req := a.pool.Get().(*Request)
	req.IP = ip
	req.Path = hashStr(path)

	select {
	case a.queue <- req:
	default:
		a.pool.Put(req)
	}
}

func (a *Analyzer) Blocked(ip string) bool {
	bl := *a.blocklist.Load()
	_, exists := bl[ip]
	return exists
}

func (a *Analyzer) Close() {
	select {
	case <-a.stop:
		return
	default:
		close(a.stop)
	}
}

func (a *Analyzer) worker() {
	ticker := time.NewTicker(a.cfg.Window)
	defer ticker.Stop()

	for {
		select {
		case <-a.stop:
			return
		case req := <-a.queue:
			a.analyze(req)
			a.pool.Put(req)
		case <-ticker.C:
			a.rotate()
		}
	}
}

func (a *Analyzer) analyze(req *Request) {
	// Bloom filter deduplication
	key := hashIPPath(req.IP, req.Path)
	if a.bloom.TestAndAdd(u64ToBytes(key)) {
		return
	}

	// Counter increment
	count := a.counter.Visit(req.IP)

	// Threshold check
	if int(count) >= a.cfg.PageThreshold {
		a.block(req.IP)
	}
}

func (a *Analyzer) block(ip string) {
	old := *a.blocklist.Load()

	if _, exists := old[ip]; exists {
		return
	}

	new := make(map[string]struct{}, len(old)+1)
	for k := range old {
		new[k] = struct{}{}
	}
	new[ip] = struct{}{}

	a.blocklist.Store(&new)
}

func (a *Analyzer) rotate() {
	a.bloom.Rotate()
	a.counter.Clear()
}

func hashIPPath(ip string, pathHash uint64) uint64 {
	var h maphash.Hash
	h.WriteString(ip)
	h.Write([]byte{
		byte(pathHash), byte(pathHash >> 8), byte(pathHash >> 16), byte(pathHash >> 24),
		byte(pathHash >> 32), byte(pathHash >> 40), byte(pathHash >> 48), byte(pathHash >> 56),
	})
	return h.Sum64()
}

func hashStr(s string) uint64 {
	var h maphash.Hash
	h.WriteString(s)
	return h.Sum64()
}

func u64ToBytes(v uint64) []byte {
	return []byte{
		byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24),
		byte(v >> 32), byte(v >> 40), byte(v >> 48), byte(v >> 56),
	}
}
