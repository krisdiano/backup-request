package main

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/Saner-Lee/backup-request/request"
	"github.com/Saner-Lee/backup-request/request/retrygroup"
	"github.com/Saner-Lee/backup-request/retry"
)

func NewServer() *httptest.Server {
	rand.Seed(time.Now().UnixNano())

	var (
		cnt int32
	)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		num := atomic.AddInt32(&cnt, 1)
		base := rand.Int63() % 20
		fmt.Printf("server: req seq %d, sleep %dms\n", num, base)
		time.Sleep(time.Duration(base) * time.Millisecond)
		w.Header().Set("seq", strconv.Itoa(int(num)))
		w.WriteHeader(http.StatusOK)
	}))
}

func main() {
	// 启动一个用于测试的http server
	srv := NewServer()
	defer srv.Close()

	// 构造原始请求
	task := func() (interface{}, error) {
		req, err := http.NewRequest(http.MethodHead, srv.URL, nil)
		if err != nil {
			panic(err)
		}
		return http.DefaultClient.Do(req)
	}
	// 请求回调处理函数
	// backup request相当于并发的重试，只接收第一次的响应
	// 非第一次返回的请求才会调用回调进行处理异常或回收资源等
	taskErrCallback := func(err error) {
		fmt.Printf("rece err: %v\n", err)
	}
	// 构造retry group，为构造backup request做准备
	// 返回的ctx也是构造backup request的参数
	rg, ctx := retrygroup.NewRetryGroup(context.Background(), task, retrygroup.WithErrHandler(taskErrCallback))

	var cnt int32
	// 重试时间算法
	event := request.WithEvent(retry.Fixed(time.Millisecond * 10))
	// 重试准入算法
	access := request.WithAccess(func(_ context.Context) bool {
		if atomic.LoadInt32(&cnt) > 5 {
			return false
		}
		atomic.AddInt32(&cnt, 1)
		return true
	})
	// 构造backup request
	req, err := request.NewRequest(ctx, rg, event, access)
	if err != nil {
		panic(err)
	}

	// 发起请求，并发的重试，阻塞等待结果
	resp, err := req.Do()
	if request.IsKilled(err) {
		fmt.Println("backup request is killed")
	}
	fmt.Printf("val is %v, err %v\n", resp, err)
}
