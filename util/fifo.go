package util

/*
算法：页面cache算法之FIFO
特点：最先进入最先淘汰
实现：链表+hashmap，hashmap用来加速查找
插入/查找/删除：O(1)
*/

import (
	"container/list"
	"errors"
	"fmt"
)

type Element struct {
	Key   string
	Value string
}

type Cache_fifo struct {
	len    int
	equeue list.List
	kmap   map[string]*list.Element
}

func (fifo *Cache_fifo) Init(len int) {
	fifo.len = len
	//fifo.equeue = list.New()
	fifo.kmap = make(map[string]*list.Element)
}

func (fifo *Cache_fifo) Get(k string) (string, error) {
	e := fifo.kmap[k]
	if e == nil {
		return "", errors.New("not found")
	}
	return e.Value.(Element).Value, nil
}

func (fifo *Cache_fifo) Del(k string) error {
	e := fifo.kmap[k]
	if e != nil {
		fifo.equeue.Remove(e)
	}
	return nil
}

func (fifo *Cache_fifo) Add(ne Element) {
	if fifo.equeue.Len() >= fifo.len {
		ef := fifo.equeue.Front()
		e := ef.Value.(Element)
		delete(fifo.kmap, e.Key)
		fifo.equeue.Remove(ef)
	}
	nep := fifo.equeue.PushBack(ne)
	fifo.kmap[ne.Key] = nep
}

func (fifo *Cache_fifo) Print() {
	for e := fifo.equeue.Front(); e != nil; e = e.Next() {
		fmt.Printf("%s/%d", e.Value.(Element).Key, e.Value.(Element).Value)
		fmt.Print("->")
	}
	fmt.Println()
}

// // test
// func main() {
//     var fifo Cache_fifo
//     fifo.init(4)
//     fifo.add(Element{"d",4})
//     fifo.add(Element{"a",1})
//     fifo.add(Element{"b",2})
//     fifo.add(Element{"c",3})
//     fifo.print()
//     fifo.add(Element{"e",5})
//     fifo.print()
// }
