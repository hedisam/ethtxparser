package eth

import (
	"github.com/hedisam/ethtxparser/internal/custompromauto"
	"github.com/prometheus/client_golang/prometheus"
)

var failedBlockRetrievals = custompromauto.Auto().NewCounter(prometheus.CounterOpts{
	Name: "ethtxparser_failed_block_retrievals_total",
	Help: "Number of failed full block retrievals",
})

var retrievedBlocks = custompromauto.Auto().NewCounter(prometheus.CounterOpts{
	Name: "ethtxparser_block_retrievals_total",
	Help: "Number of successful full block retrievals",
})

var reorgDroppedBlocks = custompromauto.Auto().NewCounter(prometheus.CounterOpts{
	Name: "ethtxparser_reorg_dropped_blocks_total",
	Help: "Number of blocks dropped from buffer due to chain reorganization",
})
