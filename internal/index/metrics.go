package index

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/hedisam/ethtxparser/internal/custompromauto"
)

var (
	blocksFailedProcessing = custompromauto.Auto().NewCounter(prometheus.CounterOpts{
		Name: "ethtxparser_blocks_failed_processing_total",
		Help: "Total number of blocks that failed processing during indexing",
	})

	processedBlocks = custompromauto.Auto().NewCounter(prometheus.CounterOpts{
		Name: "ethtxparser_blocks_processed_total",
		Help: "Total number of blocks consumed for indexing",
	})
	indexedTransactions = custompromauto.Auto().NewCounter(prometheus.CounterOpts{
		Name: "ethtxparser_indexed_transactions_total",
		Help: "Total number of transactions successfully indexed",
	})
)
