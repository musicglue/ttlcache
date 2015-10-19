package ttlcache

import (
	"container/heap"
	"sync"
)

func newPriorityQueue() *priorityQueue {
	queue := &priorityQueue{}
	heap.Init(queue)
	return queue
}

type priorityQueue struct {
	mutex sync.Mutex
	items []*item
}

func (pq *priorityQueue) update(item *item) {
	heap.Fix(pq, item.queueIndex)
}

func (pq *priorityQueue) push(item *item) {
	heap.Push(pq, item)
}

func (pq *priorityQueue) pop() *item {
	if pq.Len() == 0 {
		return nil
	}
	return heap.Pop(pq).(*item)
}

func (pq *priorityQueue) remove(item *item) {
	heap.Remove(pq, item.queueIndex)
}

func (pq priorityQueue) Len() int {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()

	length := len(pq.items)
	return length
}

func (pq priorityQueue) Less(i, j int) bool {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()

	less := pq.items[i].expireAt.Before(pq.items[j].expireAt)
	return less
}

func (pq priorityQueue) Swap(i, j int) {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()

	pq.items[i], pq.items[j] = pq.items[j], pq.items[i]
	pq.items[i].queueIndex = i
	pq.items[j].queueIndex = j
}

func (pq *priorityQueue) Push(x interface{}) {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()

	item := x.(*item)
	item.queueIndex = len(pq.items)
	pq.items = append(pq.items, item)
}

func (pq *priorityQueue) Pop() interface{} {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()

	old := pq.items
	n := len(old)
	item := old[n-1]
	item.queueIndex = -1
	pq.items = old[0 : n-1]
	return item
}
