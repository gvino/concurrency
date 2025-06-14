package concurrency_test

import (
	"math/rand/v2"
	"slices"
	"strconv"
	"testing"
	"time"

	c "github.com/gvino/concurrency"
	"github.com/stretchr/testify/assert"
)

func sliceToChan[T any](ch chan<- T, s []T) {
	for _, i := range s {
		ch <- i
	}
	close(ch)
}

func TestPipeline(t *testing.T) {
	t.Parallel()

	t.Run("int to int (square)", func(t *testing.T) {
		t.Parallel()
		c1 := make(chan int)
		c2 := c.Pipeline(func(i int) int { return i * i })(c1)

		go sliceToChan(c1, []int{0, 1, 2, 3, 4})

		res := []int{}

		for i := range c2 {
			res = append(res, i)
		}

		assert.Equal(t, []int{0, 1, 4, 9, 16}, res)
	})

	t.Run("int to string", func(t *testing.T) {
		t.Parallel()
		c1 := make(chan int)
		c2 := c.Pipeline(strconv.Itoa)(c1)

		go sliceToChan(c1, []int{0, 1, 2, 3, 4})

		res := []string{}

		for i := range c2 {
			res = append(res, i)
		}

		assert.Equal(t, []string{"0", "1", "2", "3", "4"}, res)
	})

	t.Run("shared converter", func(t *testing.T) {
		t.Parallel()
		c11 := make(chan int)
		c12 := make(chan int)
		conv := c.Pipeline(func(i int) int { return i * i })
		c21 := conv(c11)
		c22 := conv(c12)

		go sliceToChan(c11, []int{0, 1, 2, 3, 4})
		go sliceToChan(c12, []int{4, 3, 2, 1, 0})

		res1 := []int{}
		res2 := []int{}

		ok1, ok2 := true, true
		for {
			var i int
			select {
			case i, ok1 = <-c21:
				if ok1 {
					res1 = append(res1, i)
				}
			case i, ok2 = <-c22:
				if ok2 {
					res2 = append(res2, i)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("Timeout")
			}
			if !ok1 && !ok2 {
				break
			}
		}

		assert.Equal(t, []int{0, 1, 4, 9, 16}, res1)
		assert.Equal(t, []int{16, 9, 4, 1, 0}, res2)
	})
}

func TestFanIn(t *testing.T) {
	t.Parallel()

	t.Run("merge two int channels", func(t *testing.T) {
		t.Parallel()
		c1 := make(chan int)
		c2 := make(chan int)
		res := c.FanIn(c1, c2)

		go sliceToChan(c1, []int{0, 2, 4})
		go sliceToChan(c2, []int{1, 3})

		resS := []int{}
		for i := range res {
			resS = append(resS, i)
		}
		slices.Sort(resS)
		assert.Equal(t, []int{0, 1, 2, 3, 4}, resS)
	})
}

func TestBatch(t *testing.T) {
	t.Parallel()

	t.Run("batch with 1 batchSize", func(t *testing.T) {
		t.Parallel()
		c1 := make(chan int)
		c2 := c.Batch(c1, 1, 0)

		go sliceToChan(c1, []int{0, 1, 2})

		res := <-c2
		assert.Equal(t, []int{0}, res)
		res = <-c2
		assert.Equal(t, []int{1}, res)
		res = <-c2
		assert.Equal(t, []int{2}, res)
		_, ok := <-c2
		assert.False(t, ok)
	})

	t.Run("batch without timeout", func(t *testing.T) {
		t.Parallel()
		c1 := make(chan int)
		c2 := c.Batch(c1, 3, 0)

		go sliceToChan(c1, []int{0, 1, 2, 3, 4})

		res := <-c2
		assert.Equal(t, []int{0, 1, 2}, res)
		res = <-c2
		assert.Equal(t, []int{3, 4}, res)
		_, ok := <-c2
		assert.False(t, ok)
	})

	t.Run("batch with timeout", func(t *testing.T) {
		t.Parallel()
		c1 := make(chan int)
		c2 := c.Batch(c1, 3, 80*time.Millisecond)

		go func() {
			for i := range 5 {
				c1 <- i
			}
			time.Sleep(100 * time.Millisecond)

			c1 <- 5

			close(c1)
		}()

		res := <-c2
		assert.Equal(t, []int{0, 1, 2}, res)
		res = <-c2
		assert.Equal(t, []int{3, 4}, res)
		res = <-c2
		assert.Equal(t, []int{5}, res)
		_, ok := <-c2
		assert.False(t, ok)
	})
}

func TestParallel(t *testing.T) {
	t.Parallel()

	t.Run("parallel executing without closing", func(t *testing.T) {
		t.Parallel()

		tasks := make(chan int)
		fn := func(i int) int {
			time.Sleep(time.Duration(rand.IntN(100)) * time.Millisecond)

			return i * i
		}

		out := c.Parallel(tasks, fn, 3, make(chan struct{}))
		go sliceToChan(tasks, []int{0, 1, 2, 3, 4})

		res := []int{}
		for i := range out {
			res = append(res, i)
		}
		slices.Sort(res)
		assert.Equal(t, []int{0, 1, 4, 9, 16}, res)
	})

	t.Run("parallel executing with closing", func(t *testing.T) {
		t.Parallel()

		tasks := make(chan int)
		fn := func(i int) int {
			time.Sleep(time.Duration(rand.IntN(100)) * time.Millisecond)

			return i * i
		}

		done := make(chan struct{})
		out := c.Parallel(tasks, fn, 3, done)
		go func() {
			for i, t := range []int{0, 1, 2, 3, 4} {
				tasks <- t
				if i == 2 {
					close(done)
					time.Sleep(100 * time.Millisecond)
				}
			}
		}()

		res := []int{}
		for i := range out {
			res = append(res, i)
		}

		slices.Sort(res)
		assert.Equal(t, []int{0, 1, 4}, res)
	})
}

func TestChannelify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []int
	}{
		{
			name: "empty slice",
			in:   []int{},
		},
		{
			name: "nil slice",
		},
		{
			name: "single element",
			in:   []int{1},
		},
		{
			name: "multiple elements",
			in:   []int{1, 2, 3, 4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ch := c.Channelify(tt.in)

			res := []int{}

			for {
				select {
				case i, ok := <-ch:
					if !ok {
						if tt.in == nil {
							assert.Equal(t, []int{}, res)
						} else {
							assert.Equal(t, tt.in, res)
						}
						return
					}
					res = append(res, i)
				case <-time.After(1 * time.Second):
					t.Fatal("Channelify timed out")
				}
			}
		})
	}
}

func TestSlicify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		ch     chan int
		values []int
	}{
		{
			name: "nil channel",
		},
		{
			name:   "empty channel",
			ch:     make(chan int),
			values: []int{},
		},
		{
			name:   "single element",
			ch:     make(chan int),
			values: []int{1},
		},
		{
			name:   "multiple elements",
			ch:     make(chan int),
			values: []int{1, 2, 3, 4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			go func() {
				for _, r := range tt.values {
					tt.ch <- r
				}
				if tt.ch != nil {
					close(tt.ch)
				}
			}()

			res := c.Slicify(tt.ch)
			assert.Equal(t, tt.values, res)
		})
	}
}
