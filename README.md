# Simple library to make work with channels & concurrency less painfull

This library is inspired by [article (russian)](https://habr.com/ru/companies/vktech/articles/904046/).

> [!IMPORTANT]
> This library is under development

## Functions

### Pipeline

Converts channel of type IN to channel of type OUT using provided converter
function. Closes OUT channel when IN channel closed and fully consumed.

Example usage:
```go
	c1 := make(chan int)
	conv := Pipeline(func(i int) int { return i * i })
	c2 := conv(c1)

	go func() {
		for i := 0; i < 5; i++ {
			c1 <- i
		}
		close(c1)
	}()

	for i := range c2 {
		fmt.Printf("%d ", i)
	}
	fmt.Println()
```

### FanIn

Merges multiple channels into single channel. When all input channels closed and
fully consumed, output channel also closes.

Example usage:
```go
	c1 := make(chan int)
	c2 := make(chan int)
	res := FanIn(c1, c2)

	go func() {
		for i := 0; i < 5; i += 2 {
			c1 <- i
		}
		close(c1)
	}()
	go func() {
		for i := 1; i < 5; i += 2 {
			c2 <- i
		}
		close(c2)
	}()

	resS := []int{}
	for i := range res {
	  resS = append(resS, i)
	}
	slices.Sort(resS)
	fmt.Println(resS)
```

### Batch

Converts channel of type T to channel of type []T, which will be populated with
slices of `batchSize` elements from input channel. When input channel closes,
rest of it elements dumps to out channel and out channel closes. If `timeout` is
greater than 0 batches will be sent to out channel if period from last send is
greater than `timeout`.

Example usage:
```go
	c1 := make(chan int)
	c2 := Batch(c1, 3, 80 * time.Millisecond)

	go func() {
		for i := 1; i < 5; i++ {
			time.Sleep(40 * time.Millisecond)
			c1 <- i
		}
		close(c1)
	}()

	for res := range c2 {
		fmt.Println(res)
	}
```

### Parallel

Creates worker pool with `count` workers with graceful shutdown, that will
process `tasks` channel in parallel with function `fn`. Output values will be
sent to output channel. If `done` channel closes, worker pool shuts down and
closes output channel. When `tasks` channel closes, worker pool also shuts down
and closes output channel.

Example usage:
```go
	tasks := make(chan int)
	fn := func(i int) int {
		time.Sleep(time.Duration(rand.IntN(100)) * time.Millisecond)

		return i * i
	}

	done := make(chan struct{})
	out := Parallel(tasks, fn, 3, done)
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
	fmt.Println(res)
```
