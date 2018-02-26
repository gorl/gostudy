package main

import (
	"sort"
	"strconv"
	"strings"
	"sync"
	"runtime"
	"time"
)

const (
	MD5_PER_TIME    = 1
	MULTI_HASH_SIZE = 6
	BUF_SIZE = 16
)

func ExecutePipeline(jobs ...job) {
	wg := &sync.WaitGroup{}
	in := make(chan interface{}, BUF_SIZE)
	for _, actualJob := range jobs {
		out := make(chan interface{}, BUF_SIZE)
		wg.Add(1)
		go func(job job, in, out chan interface{}) {
			defer wg.Done()
			defer close(out)
			job(in, out)
		}(actualJob, in, out)
		in = out
	}
	wg.Wait()
}

var md5Limiter = make(chan struct{}, MD5_PER_TIME)

func calcCrc32(data string) chan string {
	result := make(chan string, 1)
	go func(out chan<- string) {
		out <- DataSignerCrc32(data)
	}(result)
	return result
}

func calcMd5(data string) chan string {
	result := make(chan string, 1)
	go func(out chan<- string) {
		md5Limiter <- struct{}{}
		out <- DataSignerMd5(data)
		<-md5Limiter
	}(result)
	return result
}

func toString(o interface{}) string {
	switch o.(type) {
	case int:
		return strconv.Itoa(o.(int))
	case string:
		str, _ := o.(string)
		return str
	default:
		panic("unexpected type")
	}
}

func CombineResults(in, out chan interface{}) {
	parts := make([]string, 0, BUF_SIZE)
	for data := range in {
		parts = append(parts, toString(data))
	}
	sort.Strings(parts)
	out <- strings.Join(parts, "_")
	runtime.Gosched()
}

func MultiHash(in, out chan interface{}) {
	orderer := Orderer{out:out}
	for data := range in {
		s := toString(data)
		orderer.doAsync(func(data string) string {
			outputs := make([]chan string, 0, MULTI_HASH_SIZE)
			for i := 0; i < MULTI_HASH_SIZE; i++ {
				outputs = append(outputs, calcCrc32(strconv.Itoa(i)+s))
			}
			s = ""
			for _, part := range outputs {
				s += <-part
			}
			return s
		}, s)
		runtime.Gosched()
	}
	orderer.Wait()
}

func SingleHash(in, out chan interface{}) {
	orderer := Orderer{out:out}
	for data := range in {
		s := toString(data)
		orderer.doAsync(func(data string) string {
			ch1 := calcCrc32(data)
			ch2 := calcCrc32(<-calcMd5(data))
			return <-ch1 + "~" + <-ch2
		}, s)
		runtime.Gosched()
	}
	orderer.Wait()
}

type Orderer struct {
	out chan interface{}
	mutex sync.Mutex
	sync, expected int64
	wg sync.WaitGroup
}


func (self *Orderer) doAsync(callback Callback, data string) {
	self.wg.Add(1)
	go func(s string, i int64) {
		defer self.wg.Done()
		func (expected int64) {
			r := callback(data)
		LOOP:
			for {
				shouldSchedule := false
				self.mutex.Lock()
				if self.sync == expected {
					self.out <- r
					self.sync++
				} else {
					shouldSchedule = true
				}
				self.mutex.Unlock()
				if shouldSchedule {
					time.Sleep(25 * time.Millisecond)
					//runtime.Gosched() // todo remove
				} else {
					break LOOP
				}
			}
		}(i)
	}(data, self.expected)
	self.expected++
}

func (self *Orderer) Wait() {
	self.wg.Wait()
}

type Callback func(data string) string
