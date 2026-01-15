package indexer

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shinzonetwork/shinzo-indexer-client/pkg/defra"
	"github.com/shinzonetwork/shinzo-indexer-client/pkg/logger"
)

// BlockResult holds the result of processing a block
type BlockResult struct {
	BlockNum int64
	BlockID  string
	Success  bool
	Error    error
}

// ConcurrentBlockProcessor processes multiple blocks concurrently
type ConcurrentBlockProcessor struct {
	blockHandler *defra.BlockHandler
	workers      int
	resultChan   chan *BlockResult
	pendingMu    sync.Mutex
	pending      map[int64]*BlockResult
	nextToCommit int64
}

// NewConcurrentBlockProcessor creates a new concurrent processor
func NewConcurrentBlockProcessor(blockHandler *defra.BlockHandler, workers int) *ConcurrentBlockProcessor {
	return &ConcurrentBlockProcessor{
		blockHandler: blockHandler,
		workers:      workers,
		resultChan:   make(chan *BlockResult, workers*2),
		pending:      make(map[int64]*BlockResult),
	}
}

// ProcessBlocks processes prefetched blocks concurrently while maintaining order
func (p *ConcurrentBlockProcessor) ProcessBlocks(
	ctx context.Context,
	prefetcher *BlockPrefetcher,
	startBlock int64,
	onBlockProcessed func(blockNum int64),
) error {
	p.nextToCommit = startBlock

	var wg sync.WaitGroup
	workChan := make(chan *PrefetchedBlock, p.workers)

	for i := 0; i < p.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for prefetched := range workChan {
				result := p.processBlock(ctx, prefetched)
				select {
				case p.resultChan <- result:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	var collectWg sync.WaitGroup
	collectWg.Add(1)
	go func() {
		defer collectWg.Done()
		for result := range p.resultChan {
			p.pendingMu.Lock()
			p.pending[result.BlockNum] = result

			for {
				next, ok := p.pending[p.nextToCommit]
				if !ok {
					break
				}
				delete(p.pending, p.nextToCommit)

				if next.Success {
					if next.BlockID != "existing" {
						logger.Sugar.Infof("Committed block %d (ID: %s)", next.BlockNum, next.BlockID)
					} else {
						logger.Sugar.Infof("Block %d already existed, skipping", next.BlockNum)
					}
					if onBlockProcessed != nil {
						onBlockProcessed(next.BlockNum)
					}
				} else {
					logger.Sugar.Warnf("Block %d failed: %v", next.BlockNum, next.Error)
				}
				p.nextToCommit++
			}
			p.pendingMu.Unlock()
		}
	}()

	// Start requesting blocks after the ones already seeded by the prefetcher
	nextBlockToRequest := startBlock + int64(prefetcher.bufferSize)
	processedCount := 0

	for {
		select {
		case <-ctx.Done():
			close(workChan)
			wg.Wait()
			close(p.resultChan)
			collectWg.Wait()
			return ctx.Err()
		default:
			prefetched := prefetcher.GetNext()
			if prefetched == nil {
				continue
			}

			if prefetched.Error != nil {
				if strings.Contains(prefetched.Error.Error(), "not found") ||
					strings.Contains(prefetched.Error.Error(), "does not exist") {
					logger.Sugar.Infof("Block %d not available yet, waiting...", prefetched.BlockNum)
					time.Sleep(3 * time.Second)
					prefetcher.RequestBlock(prefetched.BlockNum)
					continue
				}
				p.resultChan <- &BlockResult{
					BlockNum: prefetched.BlockNum,
					Error:    prefetched.Error,
				}
				prefetcher.RequestBlock(prefetched.BlockNum)
				continue
			}

			select {
			case workChan <- prefetched:
				processedCount++
				prefetcher.RequestBlock(nextBlockToRequest)
				nextBlockToRequest++
			case <-ctx.Done():
				close(workChan)
				wg.Wait()
				close(p.resultChan)
				collectWg.Wait()
				return ctx.Err()
			}
		}
	}
}

// processBlock processes a single prefetched block with retry on transaction conflicts
func (p *ConcurrentBlockProcessor) processBlock(ctx context.Context, prefetched *PrefetchedBlock) *BlockResult {
	result := &BlockResult{BlockNum: prefetched.BlockNum}

	const maxRetries = 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		blockID, err := p.blockHandler.CreateBlockBatch(
			ctx,
			prefetched.Block,
			prefetched.Transactions,
			prefetched.Receipts,
		)
		if err == nil {
			result.Success = true
			result.BlockID = blockID
			return result
		}

		if strings.Contains(err.Error(), "already exists") {
			result.Success = true
			result.BlockID = "existing"
			return result
		}

		// Retry on transaction conflict
		if strings.Contains(err.Error(), "transaction conflict") {
			if attempt < maxRetries-1 {
				logger.Sugar.Infof("Block %d transaction conflict, retrying (attempt %d/%d)", prefetched.BlockNum, attempt+1, maxRetries)
				time.Sleep(time.Duration(attempt+1) * 50 * time.Millisecond) // 50ms, 100ms, 150ms backoff
				continue
			}
		}

		result.Error = fmt.Errorf("failed to create block batch: %w", err)
		return result
	}

	return result
}

// GetPendingBlocks returns currently pending block numbers (for debugging)
func (p *ConcurrentBlockProcessor) GetPendingBlocks() []int64 {
	p.pendingMu.Lock()
	defer p.pendingMu.Unlock()
	blocks := make([]int64, 0, len(p.pending))
	for blockNum := range p.pending {
		blocks = append(blocks, blockNum)
	}
	sort.Slice(blocks, func(i, j int) bool { return blocks[i] < blocks[j] })
	return blocks
}
