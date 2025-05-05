package eth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/sirupsen/logrus"

	"github.com/hedisam/pipeline/chans"
)

const (
	getCurrentBlockNumber rpcMethod = "eth_blockNumber"
	getBlockByNumberID    rpcMethod = "eth_getBlockByNumber"
)

var (
	// ErrNotFound is returned when we request a block by number that hasn't been minted yet
	ErrNotFound = errors.New("block is not minted")
)

type Client struct {
	logger     *logrus.Logger
	httpClient *http.Client
	nodeAddr   string
}

func New(logger *logrus.Logger, httpClient *http.Client, nodeAddr string) *Client {
	return &Client{
		logger:     logger,
		httpClient: httpClient,
		nodeAddr:   nodeAddr,
	}
}

func (c *Client) Stream(ctx context.Context, pollTick time.Duration) <-chan *Block {
	out := make(chan *Block)

	go func() {
		defer close(out)

		t := time.NewTicker(pollTick)
		defer t.Stop()

		currentBlockNumber := int64(-2) // first time it'll be mapped to the 'latest' block number
		for range chans.ReceiveOrDoneSeq(ctx, t.C) {
			block, err := c.getFullBlock(ctx, currentBlockNumber+1)
			if err != nil {
				if errors.Is(err, ErrNotFound) {
					continue
				}
				c.logger.WithError(err).Error("Failed to get latest full block")
				failedBlockRetrievals.Inc()
				continue
			}

			if block.Number == currentBlockNumber {
				c.logger.WithField("current_block_number", block.Number).Debug("No new block yet")
				continue
			}

			c.logger.WithFields(logrus.Fields{
				"number": block.Number,
				"hash":   block.Hash,
			}).Debug("Received block")
			if !chans.SendOrDone(ctx, out, block) {
				return
			}
			currentBlockNumber = block.Number
			retrievedBlocks.Inc()
		}
	}()

	return out
}

func (c *Client) getFullBlock(ctx context.Context, blockNum int64) (*Block, error) {
	var requestedBlockNumber string
	switch blockNum {
	case -1:
		requestedBlockNumber = "latest"
	default:
		requestedBlockNumber = "0x" + strconv.FormatInt(blockNum, 16)
	}

	// last param is 'true' to request full block details
	req, err := c.newRequest(ctx, getBlockByNumberID, requestedBlockNumber, true)
	if err != nil {
		return nil, fmt.Errorf("create new http request: %w", err)
	}

	resp, err := c.doRequestWithRetry(req, "getFullBlock")
	if err != nil {
		return nil, fmt.Errorf("do request with retry: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.logger.WithField("response", string(body)).Error("Failed to get full block from eth node with unexpected status code")
		return nil, fmt.Errorf("received unexpected status: %s", resp.Status)
	}

	type Response struct {
		Block *Block `json:"result"`
	}
	var response Response
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return nil, fmt.Errorf("decode response body: %w", err)
	}

	if response.Block == nil {
		return nil, ErrNotFound
	}

	return response.Block, nil
}

func (c *Client) newRequest(ctx context.Context, method rpcMethod, rpcParams ...any) (*http.Request, error) {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  rpcParams,
		"id":      method.ID(),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("could not marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.nodeAddr, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("could ot make new request with ocntext: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", strconv.Itoa(len(data)))

	return req, nil
}

func (c *Client) doRequestWithRetry(req *http.Request, method string) (*http.Response, error) {
	bk := newExponentialBackoffConfig()
	resp, err := backoff.RetryWithData[*http.Response](func() (*http.Response, error) {
		resp, err := c.httpClient.Do(req)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				return nil, backoff.Permanent(fmt.Errorf("could not make http call: %w", err))
			}
			c.logger.WithField("method", method).WithError(err).Error("Failed to make http request, retrying...")
			return nil, fmt.Errorf("http request failed: %w", err)
		}
		return resp, nil
	}, bk)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func newExponentialBackoffConfig() *backoff.ExponentialBackOff {
	return backoff.NewExponentialBackOff(
		backoff.WithMaxElapsedTime(time.Second*3),
		backoff.WithMaxInterval(time.Second),
		backoff.WithInitialInterval(time.Millisecond*100),
		backoff.WithMultiplier(2),
		backoff.WithRandomizationFactor(0.2),
	)
}
