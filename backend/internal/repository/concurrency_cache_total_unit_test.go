//go:build unit

package repository

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTotalAccountConcurrencyFromSnapshot_IgnoresIndexMutationBetweenChunks(t *testing.T) {
	liveIndex := make([]string, 0, totalAccountConcurrencyBatchSize+1)
	for accountID := 1; accountID <= totalAccountConcurrencyBatchSize+1; accountID++ {
		liveIndex = append(liveIndex, strconv.Itoa(accountID))
	}

	snapshotReads := 0
	batchCalls := 0
	processedBatches := make([][]int64, 0, 2)
	total, err := totalAccountConcurrencyFromSnapshot(
		context.Background(),
		func(context.Context) ([]string, error) {
			snapshotReads++
			return append([]string(nil), liveIndex...), nil
		},
		func(_ context.Context, accountIDs []int64) (map[int64]int, error) {
			batchCalls++
			processedBatches = append(processedBatches, append([]int64(nil), accountIDs...))
			if batchCalls == 1 {
				liveIndex = []string{"9999"}
			}

			counts := make(map[int64]int, len(accountIDs))
			for _, accountID := range accountIDs {
				counts[accountID] = 1
			}
			return counts, nil
		},
	)

	require.NoError(t, err)
	require.Equal(t, totalAccountConcurrencyBatchSize+1, total)
	require.Equal(t, 1, snapshotReads)
	require.Equal(t, 2, batchCalls)
	require.Len(t, processedBatches[0], totalAccountConcurrencyBatchSize)
	require.Equal(t, []int64{totalAccountConcurrencyBatchSize + 1}, processedBatches[1])
}
