package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMemoryUpdateCache(t *testing.T) {
	ctx := context.Background()
	c := NewMemoryUpdateCache()

	require.NoError(t, c.PutUpdate(ctx, Update{Key: "a", GroupID: 1, Bonus: 3}))
	require.NoError(t, c.PutUpdate(ctx, Update{Key: "b", GroupID: 1, Bonus: 5}))
	require.NoError(t, c.PutUpdate(ctx, Update{Key: "c", GroupID: 2, Bonus: 7}))

	group1, err := c.GetUpdates(ctx, 1)
	require.NoError(t, err)
	require.Len(t, group1, 2)

	require.NoError(t, c.DeleteUpdate(ctx, "a"))
	group1, err = c.GetUpdates(ctx, 1)
	require.NoError(t, err)
	require.Len(t, group1, 1)
	require.Equal(t, "b", group1[0].Key)

	// перезапись по тому же ключу
	require.NoError(t, c.PutUpdate(ctx, Update{Key: "b", GroupID: 1, Bonus: 9}))
	group1, _ = c.GetUpdates(ctx, 1)
	require.Len(t, group1, 1)
	require.Equal(t, 9, group1[0].Bonus)
}
