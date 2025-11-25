package internal

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQueue(t *testing.T) {
	q := NewQueue()

	wg := sync.WaitGroup{}
	counter := 0
	incrementer := func() {
		counter++
		wg.Done()
	}

	wg.Add(2)
	q.Add(incrementer)
	q.Add(incrementer)

	wg.Wait()

	require.Equal(t, 2, counter)
}
