package analyzer

import (
	"container/list"
)

type Counter struct {
	maxSize int
	data    map[string]uint16
	lru     *list.List
	index   map[string]*list.Element
}

func NewCounter() *Counter {
	return &Counter{
		maxSize: 100000,
		data:    make(map[string]uint16),
		lru:     list.New(),
		index:   make(map[string]*list.Element),
	}
}

func (c *Counter) Visit(ip string) uint16 {
	if elem, exists := c.index[ip]; exists {
		count := c.data[ip] + 1
		c.data[ip] = count
		c.lru.MoveToFront(elem)
		return count
	}

	if len(c.data) >= c.maxSize {
		if tailElem := c.lru.Back(); tailElem != nil {
			tailIP := tailElem.Value.(string)
			delete(c.data, tailIP)
			delete(c.index, tailIP)
			c.lru.Remove(tailElem)
		}
	}

	elem := c.lru.PushFront(ip)
	c.data[ip] = 1
	c.index[ip] = elem
	return 1
}

func (c *Counter) Count(ip string) uint16 {
	return c.data[ip]
}

func (c *Counter) Clear() {
	c.data = make(map[string]uint16)
	c.lru = list.New()
	c.index = make(map[string]*list.Element)
}
