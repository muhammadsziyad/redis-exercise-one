package exerciseone

import (
	"context"
	"strconv"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestClient creates a Redis client connected to a miniredis instance
func newTestClient(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err, "failed to start miniredis")

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		client.Close()
		mr.Close()
	})
	return client, mr
}

// TestInsertValues verifies that 100 integers are correctly inserted into the Sorted Set
func TestInsertValues(t *testing.T) {
	client, _ := newTestClient(t)
	ctx := context.Background()

	err := InsertValues(ctx, client)
	require.NoError(t, err, "InsertValues should not return an error")

	// Verify total count
	count, err := client.ZCard(ctx, SortedSetKey).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(100), count, "should have exactly 100 members")

	// Verify first element (lowest score = 1)
	// ZRangeArgsWithScores replaces deprecated ZRangeWithScores
	first, err := client.ZRangeArgsWithScores(ctx, redis.ZRangeArgs{
		Key: SortedSetKey, Start: 0, Stop: 0,
	}).Result()
	require.NoError(t, err)
	require.Len(t, first, 1)
	assert.Equal(t, float64(1), first[0].Score, "first member score should be 1")
	assert.Equal(t, "1", first[0].Member, "first member should be '1'")

	// Verify last element (highest score = 100)
	last, err := client.ZRangeArgsWithScores(ctx, redis.ZRangeArgs{
		Key: SortedSetKey, Start: -1, Stop: -1,
	}).Result()
	require.NoError(t, err)
	require.Len(t, last, 1)
	assert.Equal(t, float64(100), last[0].Score, "last member score should be 100")
	assert.Equal(t, "100", last[0].Member, "last member should be '100'")
}

// TestInsertValuesIdempotent verifies that calling InsertValues twice does not duplicate entries
func TestInsertValuesIdempotent(t *testing.T) {
	client, _ := newTestClient(t)
	ctx := context.Background()

	require.NoError(t, InsertValues(ctx, client))
	require.NoError(t, InsertValues(ctx, client)) // second call

	count, err := client.ZCard(ctx, SortedSetKey).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(100), count, "idempotent insert should still result in 100 members")
}

// TestReadReverse verifies that values are returned in descending order (100 → 1)
func TestReadReverse(t *testing.T) {
	client, _ := newTestClient(t)
	ctx := context.Background()

	require.NoError(t, InsertValues(ctx, client))

	values, err := ReadReverse(ctx, client)
	require.NoError(t, err, "ReadReverse should not return an error")
	require.Len(t, values, 100, "should return exactly 100 values")

	// First value must be 100
	assert.Equal(t, "100", values[0], "first value in reverse order should be 100")

	// Last value must be 1
	assert.Equal(t, "1", values[99], "last value in reverse order should be 1")

	// Verify strictly descending order throughout
	for i := 0; i < len(values)-1; i++ {
		curr, err := strconv.Atoi(values[i])
		require.NoError(t, err)
		next, err := strconv.Atoi(values[i+1])
		require.NoError(t, err)
		assert.Greater(t, curr, next,
			"value at index %d (%d) should be greater than value at index %d (%d)",
			i, curr, i+1, next)
	}
}

// TestReadReverseEmptyKey verifies behavior when the key does not exist
func TestReadReverseEmptyKey(t *testing.T) {
	client, _ := newTestClient(t)
	ctx := context.Background()

	values, err := ReadReverse(ctx, client)
	require.NoError(t, err, "ReadReverse on empty key should not error")
	assert.Empty(t, values, "result should be empty for non-existent key")
}

// TestVerifyReplication simulates replication by manually syncing data
// between two miniredis instances and verifying count equality
func TestVerifyReplication(t *testing.T) {
	srcClient, _ := newTestClient(t)
	repClient, _ := newTestClient(t)
	ctx := context.Background()

	// Initially not replicated
	ok, err := VerifyReplication(ctx, srcClient, repClient)
	require.NoError(t, err)
	assert.False(t, ok, "replication should not be verified before data is synced")

	// Insert into source
	require.NoError(t, InsertValues(ctx, srcClient))

	// Simulate replication: copy data from source to replica
	// ZRangeArgsWithScores replaces deprecated ZRangeWithScores
	members, err := srcClient.ZRangeArgsWithScores(ctx, redis.ZRangeArgs{
		Key: SortedSetKey, Start: 0, Stop: -1,
	}).Result()
	require.NoError(t, err)
	repClient.ZAdd(ctx, SortedSetKey, members...)

	// Now verify
	ok, err = VerifyReplication(ctx, srcClient, repClient)
	require.NoError(t, err)
	assert.True(t, ok, "replication should be verified after data is synced")
}

// TestSortedSetScores verifies that scores equal their integer values
func TestSortedSetScores(t *testing.T) {
	client, _ := newTestClient(t)
	ctx := context.Background()

	require.NoError(t, InsertValues(ctx, client))

	// Check a sample of score-to-member relationships
	testCases := []struct {
		member string
		score  float64
	}{
		{"1", 1.0},
		{"50", 50.0},
		{"100", 100.0},
	}

	for _, tc := range testCases {
		score, err := client.ZScore(ctx, SortedSetKey, tc.member).Result()
		require.NoError(t, err)
		assert.Equal(t, tc.score, score, "member '%s' should have score %.0f", tc.member, tc.score)
	}
}
