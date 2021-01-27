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
	Key   interface{}
	Value interface{}
}

type Cache_fifo struct {
	len    int
	equeue list.List
	kmap   map[interface{}]*list.Element
}

func (fifo *Cache_fifo) Init(len int) {
	fifo.len = len
	//fifo.equeue = list.New()
	fifo.kmap = make(map[interface{}]*list.Element)
}

func (fifo *Cache_fifo) Get(k interface{}) (interface{}, error) {
	e := fifo.kmap[k]
	if e == nil {
		return "", errors.New("not found")
	}
	return e.Value.(Element).Value, nil
}

func (fifo *Cache_fifo) Update(k interface{}, v interface{}) error {
	e := fifo.kmap[k]
	if e == nil {
		return errors.New("not found")
	}
	e.Value = Element{Key: k, Value: v}

	return nil
}

func (fifo *Cache_fifo) Del(k interface{}) error {
	e := fifo.kmap[k]
	if e != nil {
		fifo.equeue.Remove(e)
	}
	return nil
}

func (fifo *Cache_fifo) Add(k interface{}, v interface{}) {
	ne := Element{Key: k, Value: v}
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
		fmt.Printf("%v/%s", e.Value.(Element).Key, e.Value.(Element).Value)
		fmt.Print("->")
	}
	fmt.Println()
}

// // test
// func main() {
// 	var fifo Cache_fifo
// 	fifo.Init(4)
// 	fifo.Add(1, "4")
// 	fifo.Add(2, "1")
// 	fifo.Add(3, "2")
// 	fifo.Add("c", "3")
// 	fifo.Print()
// 	fifo.Add("e", "5")
// 	fifo.Update("c", "4")
// 	fifo.Print()
// }
