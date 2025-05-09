// Copyright 2025 George Vinogradov. All rights reserved
// Use of this source code is governed by a MIT license that
// can be found in the LICENSE file.

// Package concurrency provides functions to simplify concurrent programming.
package concurrency

import (
	"sync"
	"time"
)

// Pipeline creates converter for channels from IN to OUT types using <fn> to
// convert individual items. When input channel closes, converter closes output
// channel.
//
// Example usage:
//
//	c1 := make(chan int)
//	conv := Pipeline(func(i int) int { return i * i })
//	c2 := conv(c1)
//
//	go func() {
//		for i := 0; i < 5; i++ {
//			c1 <- i
//		}
//		close(c1)
//	}()
//
//	for i := range c2 {
//		fmt.Printf("%d ", i)
//	}
//	fmt.Println()
//
// Output:
//
//	0 1 4 9 16
func Pipeline[IN, OUT any](fn func(IN) OUT) func(in <-chan IN) <-chan OUT {
	return func(in <-chan IN) <-chan OUT {
		out := make(chan OUT)

		go func(in <-chan IN) {
			for i := range in {
				out <- fn(i)
			}
			close(out)
		}(in)

		return out
	}
}

// FanIn merges multiple channels into single channel. When all input channels
// closes, FanIn closes output channel.
//
// Example usage:
//
//	c1 := make(chan int)
//	c2 := make(chan int)
//	res := FanIn(c1, c2)
//
//	go func() {
//		for i := 0; i < 5; i += 2 {
//			c1 <- i
//		}
//		close(c1)
//	}()
//	go func() {
//		for i := 1; i < 5; i += 2 {
//			c2 <- i
//		}
//		close(c2)
//	}()
//
//	resS := []int{}
//	for i := range res {
//	  resS = append(resS, i)
//	}
//	slices.Sort(resS)
//	fmt.Println(resS)
//
// Output:
//
//	0 1 2 3 4
func FanIn[T any](streams ...<-chan T) <-chan T {
	out := make(chan T)
	var wg sync.WaitGroup

	wg.Add(len(streams))
	for _, c := range streams {
		go func(c <-chan T) {
			for i := range c {
				out <- i
			}
			wg.Done()
		}(c)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

// Batch takes channel, batchSize and timeout and returns channel with slices of
// elements. If timeout <= 0 then it waits full batch of items from channel <ch>
// or closing that channel and sends batches to out channel.
// If timeout greater than 0, buffer is sent to out channel if it is full or if
// timeout reached after last send.
// When input channel closed, Batch closes output channel.
//
// Example usage:
//
//	c1 := make(chan int)
//	c2 := Batch(c1, 3, 80 * time.Millisecond)
//
//	go func() {
//		for i := 1; i < 5; i++ {
//			time.Sleep(40 * time.Millisecond)
//			c1 <- i
//		}
//		close(c1)
//	}()
//
//	for res := range c2 {
//		fmt.Println(res)
//	}
//
// Output:
//
//	[0, 1]
//	[2, 3]
//	[4]
//
//nolint:gocognit,cyclop // Don't know at this time how to simplify more
func Batch[T any](ch <-chan T, batchSize int, timeout time.Duration) <-chan []T {
	out := make(chan []T)

	go func() {
		for batchSize <= 1 {
			for i := range ch {
				out <- []T{i}
			}
		}

		var ticker *time.Ticker
		var tm <-chan time.Time
		if timeout > 0 {
			ticker = time.NewTicker(timeout)
			defer ticker.Stop()
			tm = ticker.C
		}

		for {
			buf := make([]T, 0, batchSize)
			timerBreak := false

			for range batchSize {
				select {
				case i, ok := <-ch:
					if !ok {
						if len(buf) > 0 {
							out <- buf
						}
						close(out)
						return
					}
					buf = append(buf, i)

				case <-tm:
					timerBreak = true
				}

				if timerBreak {
					break
				}
			}

			if len(buf) > 0 {
				out <- buf
			}
			if ticker != nil {
				ticker.Reset(timeout)
			}
		}
	}()

	return out
}

// Parallel takes channel with tasks, worker <fn>, count of parallel workers and
// done channel and performs work on tasks in <count> goroutines using <fn>.
// Result of computations is sent to out channel.
// Output channel is closed after closing input channel and all tasks in it
// completes.
// When done channel closed all workers stops after finishing current task and
// output channel is closed.
//
// Example usage:
//
//	tasks := make(chan int)
//	fn := func(i int) int {
//		time.Sleep(time.Duration(rand.IntN(100)) * time.Millisecond)
//
//		return i * i
//	}
//
//	done := make(chan struct{})
//	out := Parallel(tasks, fn, 3, done)
//	go func() {
//		for i, t := range []int{0, 1, 2, 3, 4} {
//			tasks <- t
//			if i == 2 {
//				close(done)
//				time.Sleep(100 * time.Millisecond)
//			}
//		}
//	}()
//
//	res := []int{}
//	for i := range out {
//		res = append(res, i)
//	}
//
//	slices.Sort(res)
//	fmt.Println(res)
//
// Output:
//
//	[0, 1, 4]
func Parallel[IN, OUT any](tasks <-chan IN, fn func(IN) OUT, count int, done <-chan struct{}) <-chan OUT {
	out := make(chan OUT)
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()

		for {
			select {
			case task, ok := <-tasks:
				if !ok {
					return
				}
				out <- fn(task)
			case <-done:
				return
			}
		}
	}

	wg.Add(count)
	for range count {
		go worker()
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}
