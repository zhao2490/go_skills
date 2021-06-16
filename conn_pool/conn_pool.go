package conn_pool

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// 实现一个简化版连接池
// todo: 连接池连接需要有存活时间，并在连接过期的时候从连接池删除 这个还没写

type Conn struct {
	maxConn       int                     // 最大连接数
	maxIdle       int                     // 最大可用连接数
	freeConn      int                     // 线程池空闲连接数
	connPool      []int                   // 连接池
	openCount     int                     // 已经打开的连接数
	waitConn      map[int]chan Permission // 排队等待的连接队列
	waitCount     int                     // 等待个数
	lock          sync.Mutex              // 锁
	nextConnIndex NextConnIndex           // 下一个连接的ID标识（用于区分每个ID）
	freeConns     map[int]Permission      // 连接池的连接
}

// 拿到一个非空的Permission才有资格执行后面类似增删改查的操作。
type Permission struct {
	NextConnIndex               // 对应Conn中的NextConnIndex
	Content       string        // 通行证的具体内容，比如"PASSED"表示成功获取
	CreatedAt     time.Time     // 创建时间，即连接的创建时间
	MaxLifeTime   time.Duration // 连接的存活时间，本次没有用到这个属性，保留
}

type NextConnIndex struct {
	Index int
}

type Config struct {
	MaxConn int
	MaxIdle int
}

// 新建一个连接池
func Prepare(ctx context.Context, config *Config) (conn *Conn) {
	return &Conn{
		maxConn:   config.MaxConn,
		maxIdle:   config.MaxIdle,
		openCount: 0,
		connPool:  []int{},
		waitConn:  make(map[int]chan Permission),
		waitCount: 0,
		freeConns: make(map[int]Permission),
	}
}

func (conn *Conn) New(ctx context.Context) (permission Permission, err error) {

	conn.lock.Lock()

	select {
	case <-ctx.Done():
		conn.lock.Unlock()
		return Permission{}, errors.New("new conn failed, context canceled")
	default:

	}

	// 如果链接池不为空，从链接池获取链接
	if len(conn.freeConns) > 0 {
		var popPermission Permission
		var popReqKey int

		// 获取其中一个链接
		for popReqKey, popPermission = range conn.freeConns {
			break
		}
		// 从空闲链接中删除获取到的链接
		delete(conn.freeConns, popReqKey)
		fmt.Println("log", "use free conn!!", "openCount: ", conn.openCount, " freeConns: ", conn.freeConns)
		conn.lock.Unlock()
		return popPermission, nil
	}

	nextConnIndex := getNextConnIndex(conn)
	// 当前连接数大于上限，则加入等待队列
	if conn.openCount >= conn.maxConn {

		req := make(chan Permission, 1)
		conn.waitConn[nextConnIndex] = req
		conn.waitCount++
		conn.lock.Unlock()

		select {
		// 在等待指定超时时间后，仍然无法获取到释放的链接，则放弃获取链接，这里如果不再超时时间后退出会一直阻塞
		case <-time.After(time.Second * time.Duration(3)):
			fmt.Println("log", "acquire new conn timeout")
			return Permission{}, errors.New("new conn failed, acquire timeout")
		case ret, ok := <-req:
			if !ok {
				return Permission{}, errors.New("new conn failed, no available conn release")
			}
			fmt.Println("log", "received released conn!!!", "openCount: ", conn.openCount, " freeConns: ", conn.freeConns)
			return ret, nil
		}
	}

	// 新建连接
	conn.openCount++
	conn.lock.Unlock()
	permission = Permission{NextConnIndex: NextConnIndex{nextConnIndex},
		Content: "PASSED", CreatedAt: time.Now(), MaxLifeTime: time.Second * 5}
	fmt.Println("log", "create conn!!!!!", "openCount: ", conn.openCount, " freeConns: ", conn.freeConns)
	return permission, nil
}

func getNextConnIndex(conn *Conn) int {
	currentIndex := conn.nextConnIndex.Index
	conn.nextConnIndex.Index = currentIndex + 1
	return conn.nextConnIndex.Index
}

// 释放链接
func (conn *Conn) Release(ctx context.Context) (result bool, err error) {
	conn.lock.Lock()
	// 如果等待队列有等待任务，则通知正在阻塞等待获取连接的进程(New方法中 <- req 逻辑)
	// 如果没有做指定连接的释放，只是保证释放的连接会被利用起来
	if len(conn.waitConn) > 0 {
		var req chan Permission
		var reqKey int
		for reqKey, req = range conn.waitConn {
			break
		}
		// 假定释放的连接就是下面的新建的连接
		permission := Permission{NextConnIndex: NextConnIndex{reqKey},
			Content: "PASSED", CreatedAt: time.Now(), MaxLifeTime: time.Second * 5}
		req <- permission
		conn.waitCount--
		delete(conn.waitConn, reqKey)
		conn.lock.Unlock()
	} else {
		// 如果当前无等待任务，则将连接放入连接池
		if conn.openCount > 0 {
			conn.openCount--

			if len(conn.freeConns) < conn.maxIdle { // 确保链接池数量不会超过maxIdle
				nextConnIndex := getNextConnIndex(conn)
				permission := Permission{NextConnIndex: NextConnIndex{Index: nextConnIndex},
					Content: "PASSED", CreatedAt: time.Now(), MaxLifeTime: time.Second * 5}
				conn.freeConns[nextConnIndex] = permission
			}
		}
		conn.lock.Unlock()
	}
	return
}
