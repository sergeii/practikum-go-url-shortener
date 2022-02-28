package background_test

import (
	"context"
	"testing"
	"time"

	"github.com/sergeii/practikum-go-url-shortener/pkg/background"
	"github.com/stretchr/testify/assert"
)

func TestBackgroundJobProcessing(t *testing.T) {
	numbers := make([]int, 0)
	ch := make(chan int, 100)
	pool := background.NewPool(background.PoolConfig{
		Concurrency:   5,
		DoJobTimeout:  time.Millisecond * 500,
		AddJobTimeout: time.Second,
	})
	for i := 1; i < 101; i++ {
		x := i
		// nolint:errcheck
		pool.Add(context.TODO(), background.NewJob("test", func(context.Context) error {
			ch <- x
			return nil
		}))
	}

	go func() {
		<-time.After(time.Millisecond * 50)
		close(ch)
		pool.Close()
	}()

	sum := 0
	for n := range ch {
		numbers = append(numbers, n)
		sum += n
	}
	assert.Len(t, numbers, 100)
	assert.Equal(t, 5050, sum)
}
