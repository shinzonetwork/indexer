package indexer

import (
	"context"
	"math/big"
	"sync"

	"github.com/shinzonetwork/shinzo-indexer-client/pkg/logger"
	"github.com/shinzonetwork/shinzo-indexer-client/pkg/rpc"
	"github.com/shinzonetwork/shinzo-indexer-client/pkg/types"
)

// PrefetchedBlock holds a prefetched block with its receipts
type PrefetchedBlock struct {
	BlockNum     int64
	Block        *types.Block
	Transactions []*types.Transaction
	Receipts     []*types.TransactionReceipt
	Error        error
}

// BlockPrefetcher prefetches blocks and their receipts ahead of time
type BlockPrefetcher struct {
	ethClient      *rpc.EthereumClient
	bufferSize     int
	receiptWorkers int
	blockChan      chan *PrefetchedBlock
	requestChan    chan int64
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
}

// NewBlockPrefetcher creates a new block prefetcher
func NewBlockPrefetcher(ethClient *rpc.EthereumClient, bufferSize, receiptWorkers int) *BlockPrefetcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &BlockPrefetcher{
		ethClient:      ethClient,
		bufferSize:     bufferSize,
		receiptWorkers: receiptWorkers,
		blockChan:      make(chan *PrefetchedBlock, bufferSize),
		requestChan:    make(chan int64, bufferSize*2),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start begins the prefetching goroutines
func (p *BlockPrefetcher) Start(startBlock int64) {
	for i := 0; i < p.bufferSize; i++ {
		p.wg.Add(1)
		go p.prefetchWorker()
	}
	for i := 0; i < p.bufferSize; i++ {
		select {
		case p.requestChan <- startBlock + int64(i):
		case <-p.ctx.Done():
			return
		}
	}
}

// prefetchWorker fetches blocks and their receipts
func (p *BlockPrefetcher) prefetchWorker() {
	defer p.wg.Done()
	for {
		select {
		case <-p.ctx.Done():
			return
		case blockNum := <-p.requestChan:
			result := p.fetchBlockWithReceipts(blockNum)
			select {
			case p.blockChan <- result:
			case <-p.ctx.Done():
				return
			}
		}
	}
}

// fetchBlockWithReceipts fetches a block and all its transaction receipts
func (p *BlockPrefetcher) fetchBlockWithReceipts(blockNum int64) *PrefetchedBlock {
	result := &PrefetchedBlock{BlockNum: blockNum}

	block, err := p.ethClient.GetBlockByNumber(p.ctx, big.NewInt(blockNum))
	if err != nil {
		result.Error = err
		return result
	}
	result.Block = block

	transactions := make([]*types.Transaction, len(block.Transactions))
	for i := range block.Transactions {
		transactions[i] = &block.Transactions[i]
	}
	result.Transactions = transactions

	receipts := make([]*types.TransactionReceipt, len(block.Transactions))
	var wg sync.WaitGroup
	sem := make(chan struct{}, p.receiptWorkers)

	for i, tx := range block.Transactions {
		wg.Add(1)
		go func(idx int, txHash string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			receipt, err := p.ethClient.GetTransactionReceipt(p.ctx, txHash)
			if err != nil {
				logger.Sugar.Warnf("Failed to fetch receipt for tx %s: %v", txHash, err)
				return
			}
			receipts[idx] = receipt
		}(i, tx.Hash)
	}
	wg.Wait()

	validReceipts := make([]*types.TransactionReceipt, 0, len(receipts))
	for _, r := range receipts {
		if r != nil {
			validReceipts = append(validReceipts, r)
		}
	}
	result.Receipts = validReceipts

	return result
}

// GetNext returns the next prefetched block
func (p *BlockPrefetcher) GetNext() *PrefetchedBlock {
	select {
	case block := <-p.blockChan:
		return block
	case <-p.ctx.Done():
		return nil
	}
}

// RequestBlock requests a new block to be prefetched
func (p *BlockPrefetcher) RequestBlock(blockNum int64) {
	select {
	case p.requestChan <- blockNum:
	case <-p.ctx.Done():
	}
}

// Stop stops the prefetcher
func (p *BlockPrefetcher) Stop() {
	p.cancel()
	p.wg.Wait()
	close(p.blockChan)
	close(p.requestChan)
}
