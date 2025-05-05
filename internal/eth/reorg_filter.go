package eth

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/hedisam/ethtxparser/internal/ringbuffer"
	"github.com/hedisam/pipeline/chans"
)

func ReorgFilter(ctx context.Context, logger *logrus.Logger, in <-chan *Block, confirmationDepth uint) <-chan *Block {
	out := make(chan *Block)

	go func() {
		defer close(out)

		rb := ringbuffer.New[*Block](confirmationDepth)
		for block := range chans.ReceiveOrDoneSeq(ctx, in) {
			logger := logger.WithFields(logrus.Fields{
				"block_hash":  block.Hash,
				"parent_hash": block.ParentHash,
			})
			// check if reorg has happened
			for rb.Size() > 0 {
				tail, _ := rb.Back()
				if block.ParentHash == tail.Hash {
					// no reorg; we're good to go
					break
				}
				// reorg has happened; discard the items in the queue until we either reach the legit block that has
				// a hash matching the newly received block's parentHash, or we have dropped all the queued items and
				// end up with this newly received block as the only one in the queue.
				logger.WithField("tail_hash", tail.Hash).Warn("Block reorganisation detected, dropping last queued non matching block")
				rb.DropBack()
				reorgDroppedBlocks.Inc()
			}

			if rb.IsFull() {
				// pop the oldest block and send it to the output channel before pushing this new block
				first, _ := rb.Pop()
				if !chans.SendOrDone(ctx, out, first) {
					return
				}
			}

			_ = rb.Push(block)
		}
	}()

	return out
}
