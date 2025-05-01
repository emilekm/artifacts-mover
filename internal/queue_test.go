package internal

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQueue(t *testing.T) {
	q := NewQueue()

	counter := 0
	incrementer := func() error {
		counter++
		return nil
	}

	ch1 := q.Add(incrementer)
	ch2 := q.Add(incrementer)

	err := <-ch1
	require.NoError(t, err)

	err = <-ch2
	require.NoError(t, err)

	require.Equal(t, 2, counter)
}
