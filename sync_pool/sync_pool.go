package sync_pool

import "sync"

/*
问题：
实际项目基本都是通过c := make([]int, 0, l)来申请内存，长度都是不确定的。
自然而然这些变量都会申请到堆上面了。Golang使用的垃圾回收算法是『标记——清除』。
简单得说，就是程序要从操作系统申请一块比较大的内存，内存分成小块，通过链表链接。
每次程序申请内存，就从链表上面遍历每一小块，找到符合的就返回其地址，没有合适的
就从操作系统再申请。如果申请内存次数较多，而且申请的大小不固定，就会引起内存碎片化的问题。
申请的堆内存并没有用完，但是用户申请的内存的时候却没有合适的空间提供。
这样会遍历整个链表，还会继续向操作系统申请内存。申请一块内存变成了慢语句。
*/

/*
解决方案：
分配内存的时候，从池子里面找满足容量切最小的池子。比如申请长度是2的
，就分配大小为5的那个池子。如果是11，就分配大小是20的那个池子里面的对象；
如果申请的slice很大，超过了上限30000，这种情况就不使用池子了，直接从内存申请；
当然这些参数可以根据自己实际情况调整；
和之前的做法有所区别，把对象重新放回池子是通过Free方法实现的。
*/

/*
参考：
https://blog.cyeam.com/golang/2017/02/08/go-optimize-slice-pool

https://www.akshaydeo.com/blog/2017/12/23/How-did-I-improve-latency-by-700-percent-using-syncPool/
*/

var DEFAULT_SYNC_POOL *SyncPool

func NewPool() *SyncPool {
	// 为了能支持变长的slice，这里申请了多个池子，其大小是从5开始，最大到30000
	DEFAULT_SYNC_POOL = NewSyncPool(5, 30000, 2)
	return DEFAULT_SYNC_POOL
}

type SyncPool struct {
	classes     []sync.Pool
	classesSize []int
	maxSize     int
	minSize     int
}

func NewSyncPool(minSize, maxSize, factor int) *SyncPool {
	n := 0
	for chunkSize := minSize; chunkSize <= maxSize; chunkSize *= factor {
		n++
	}
	pool := &SyncPool{
		make([]sync.Pool, n),
		make([]int, n),
		minSize,
		maxSize,
	}
	n = 0
	for chunkSize := minSize; chunkSize <= maxSize; chunkSize *= factor {
		pool.classesSize[n] = chunkSize
		pool.classes[n].New = func(size int) func() interface{} {
			return func() interface{} {
				buf := make([]int64, size)
				return &buf
			}
		}(chunkSize)
		n++
	}
	return pool
}

func (pool *SyncPool) Alloc(size int) []int64 {
	for size <= pool.maxSize {
		for i := 0; i < len(pool.classesSize); i++ {
			mem := pool.classes[i].Get().(*[]int64)
			return (*mem)[:0]
		}
	}
	return make([]int64, 0, size)
}

func (pool *SyncPool) Free(mem []int64) {
	if size := cap(mem); size <= pool.maxSize {
		for i := 0; i < len(pool.classesSize); i++ {
			if pool.classesSize[i] >= size {
				pool.classes[i].Put(&mem)
				return
			}
		}
	}
}
