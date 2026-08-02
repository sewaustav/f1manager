package dispatcher

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPhaseTracker_SetGet(t *testing.T) {
	p := NewPhaseTracker()

	phase, stage, ok := p.Get(1)
	require.False(t, ok)
	require.Equal(t, "", phase)
	require.Equal(t, int64(0), stage)

	p.Set(1, PhaseDraft, 0)
	phase, stage, ok = p.Get(1)
	require.True(t, ok)
	require.Equal(t, PhaseDraft, phase)
	require.Equal(t, int64(0), stage)

	p.Set(1, PhaseRacing, 3)
	phase, stage, ok = p.Get(1)
	require.True(t, ok)
	require.Equal(t, PhaseRacing, phase)
	require.Equal(t, int64(3), stage)

	// unrelated group unaffected
	_, _, ok = p.Get(2)
	require.False(t, ok)
}

func TestPhaseTracker_Concurrent(t *testing.T) {
	p := NewPhaseTracker()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			p.Set(int64(i%5), PhaseRacing, int64(i))
			p.Get(int64(i % 5))
		}(i)
	}
	wg.Wait()
}
