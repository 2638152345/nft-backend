package indexer

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"math/big"
	"sort"
	"strings"
	"sync"
	"time"

	"nft-auction-homework/backend/internal/config"
	"nft-auction-homework/backend/internal/model"
	"nft-auction-homework/backend/internal/repository"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"gorm.io/gorm"
)

type Indexer struct {
	cfg      config.Config
	repo     *repository.Repository
	client   *ethclient.Client
	decoder  *Decoder
	contract common.Address

	mu       sync.Mutex
	nextFrom uint64
}

func New(cfg config.Config, repo *repository.Repository, client *ethclient.Client) (*Indexer, error) {
	decoder, err := NewDecoder()
	if err != nil {
		return nil, err
	}
	return &Indexer{
		cfg:      cfg,
		repo:     repo,
		client:   client,
		decoder:  decoder,
		contract: common.HexToAddress(cfg.AuctionContractAddress),
	}, nil
}

func (i *Indexer) Run(ctx context.Context) {
	if err := i.initCursor(ctx); err != nil {
		log.Printf("indexer init cursor error: %v", err)
		return
	}

	for {
		err := i.syncOnce(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("indexer sync error: %v", err)
			if waitErr := sleepWithContext(ctx, 2*time.Second); waitErr != nil {
				return
			}
			continue
		}

		if waitErr := sleepWithContext(ctx, i.cfg.PollInterval); waitErr != nil {
			return
		}
	}
}

func (i *Indexer) initCursor(ctx context.Context) error {
	state, err := i.repo.GetSyncState(ctx, i.cfg.SyncStateID)
	if err != nil {
		return err
	}

	nextFrom := i.cfg.AuctionDeployBlock
	if state != nil {
		if state.LastProcessedBlock > i.cfg.ReorgBuffer {
			nextFrom = state.LastProcessedBlock - i.cfg.ReorgBuffer
		} else {
			nextFrom = i.cfg.AuctionDeployBlock
		}
		if nextFrom < i.cfg.AuctionDeployBlock {
			nextFrom = i.cfg.AuctionDeployBlock
		}
	} else {
		seedBlock := uint64(0)
		if i.cfg.AuctionDeployBlock > 0 {
			seedBlock = i.cfg.AuctionDeployBlock - 1
		}
		if err := i.repo.UpsertSyncState(ctx, i.cfg.SyncStateID, seedBlock); err != nil {
			return err
		}
	}

	i.mu.Lock()
	i.nextFrom = nextFrom
	i.mu.Unlock()

	log.Printf("indexer cursor initialized: next_from=%d", nextFrom)
	return nil
}

func (i *Indexer) syncOnce(ctx context.Context) error {
	latest, err := i.retryBlockNumber(ctx)
	if err != nil {
		return err
	}

	if int64(latest) <= i.cfg.Confirmations {
		return nil
	}

	safeHead := latest - uint64(i.cfg.Confirmations)
	from := i.getNextFrom()
	if safeHead < from {
		return nil
	}

	for chunkStart := from; chunkStart <= safeHead; {
		chunkEnd := minUint64(chunkStart+i.cfg.BlockChunkSize-1, safeHead)
		if err := i.processChunkWithRetry(ctx, chunkStart, chunkEnd); err != nil {
			return err
		}
		if err := i.repo.UpsertSyncState(ctx, i.cfg.SyncStateID, chunkEnd); err != nil {
			return err
		}
		i.setNextFrom(chunkEnd + 1)
		chunkStart = chunkEnd + 1
	}

	return nil
}

func (i *Indexer) processChunkWithRetry(ctx context.Context, fromBlock, toBlock uint64) error {
	var attempt int
	for {
		err := i.processChunk(ctx, fromBlock, toBlock)
		if err == nil {
			return nil
		}
		attempt++
		delay := backoffDuration(attempt)
		log.Printf("indexer chunk retry from=%d to=%d attempt=%d err=%v", fromBlock, toBlock, attempt, err)
		if waitErr := sleepWithContext(ctx, delay); waitErr != nil {
			return waitErr
		}
	}
}

func (i *Indexer) processChunk(ctx context.Context, fromBlock, toBlock uint64) error {
	logs, err := i.fetchLogsWithRetry(ctx, fromBlock, toBlock)
	if err != nil {
		return err
	}

	sort.Slice(logs, func(a, b int) bool {
		if logs[a].BlockNumber == logs[b].BlockNumber {
			return logs[a].Index < logs[b].Index
		}
		return logs[a].BlockNumber < logs[b].BlockNumber
	})

	blockTimes := map[uint64]time.Time{}
	for _, lg := range logs {
		if err := i.processLogWithRetry(ctx, lg, blockTimes); err != nil {
			return err
		}
	}

	log.Printf("indexer processed chunk from=%d to=%d logs=%d", fromBlock, toBlock, len(logs))
	return nil
}

func (i *Indexer) processLogWithRetry(ctx context.Context, lg types.Log, blockTimes map[uint64]time.Time) error {
	var attempt int
	for {
		err := i.processSingleLog(ctx, lg, blockTimes)
		if err == nil {
			return nil
		}

		var decodeErr *decodeError
		if errors.As(err, &decodeErr) {
			return err
		}

		attempt++
		delay := backoffDuration(attempt)
		log.Printf("indexer log retry block=%d tx=%s idx=%d attempt=%d err=%v", lg.BlockNumber, lg.TxHash.Hex(), lg.Index, attempt, err)
		if waitErr := sleepWithContext(ctx, delay); waitErr != nil {
			return waitErr
		}
	}
}

func (i *Indexer) processSingleLog(ctx context.Context, lg types.Log, blockTimes map[uint64]time.Time) error {
	eventData, eventName, err := i.decoder.Decode(lg)
	if err != nil {
		return &decodeError{cause: err}
	}

	blockTime, err := i.getBlockTime(ctx, lg.BlockNumber, blockTimes)
	if err != nil {
		return err
	}

	txHash := strings.ToLower(lg.TxHash.Hex())
	logIndex := uint(lg.Index)

	return i.repo.WithTx(ctx, func(tx *gorm.DB) error {
		processed, err := i.repo.IsLogProcessedTx(tx, txHash, logIndex)
		if err != nil {
			return err
		}
		if processed {
			return nil
		}

		switch evt := eventData.(type) {
		case AuctionCreatedEvent:
			endTime := time.Unix(int64(evt.EndTime), 0).UTC()
			auction := model.Auction{
				ID:            evt.AuctionID,
				NFTAddress:    evt.NFTAddress,
				TokenID:       evt.TokenID,
				Seller:        evt.Seller,
				BidToken:      evt.BidToken,
				StartPrice:    evt.StartPrice,
				HighestBid:    evt.StartPrice,
				HighestBidder: nil,
				StartTime:     blockTime,
				EndTime:       endTime,
				Status:        model.AuctionStatusActive,
				CreatedBlock:  lg.BlockNumber,
				UpdatedBlock:  lg.BlockNumber,
			}
			if err := i.repo.UpsertAuctionTx(tx, auction); err != nil {
				return err
			}
		case AuctionBidEvent:
			bid := model.AuctionBid{
				AuctionID:   evt.AuctionID,
				Bidder:      evt.Bidder,
				Amount:      evt.Amount,
				TxHash:      txHash,
				LogIndex:    logIndex,
				BlockNumber: lg.BlockNumber,
				BlockTime:   blockTime,
			}
			if err := i.repo.InsertBidTx(tx, bid); err != nil {
				return err
			}
			if err := i.repo.UpdateAuctionHighestBidTx(tx, evt.AuctionID, evt.Amount, evt.Bidder, lg.BlockNumber); err != nil {
				return err
			}
		case AuctionEndedEvent:
			if err := i.repo.UpdateAuctionHighestBidTx(tx, evt.AuctionID, evt.FinalPrice, evt.Winner, lg.BlockNumber); err != nil {
				return err
			}
			if err := i.repo.UpdateAuctionStatusTx(tx, evt.AuctionID, model.AuctionStatusEnded, lg.BlockNumber); err != nil {
				return err
			}
		case AuctionCancelledEvent:
			if err := i.repo.UpdateAuctionStatusTx(tx, evt.AuctionID, model.AuctionStatusCancelled, lg.BlockNumber); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported decoded event type")
		}

		return i.repo.InsertProcessedLogTx(tx, model.ProcessedLog{
			TxHash:      txHash,
			LogIndex:    logIndex,
			EventName:   eventName,
			BlockNumber: lg.BlockNumber,
		})
	})
}

func (i *Indexer) fetchLogsWithRetry(ctx context.Context, fromBlock, toBlock uint64) ([]types.Log, error) {
	var attempt int
	for {
		logs, err := i.fetchLogs(ctx, fromBlock, toBlock)
		if err == nil {
			return logs, nil
		}
		attempt++
		delay := backoffDuration(attempt)
		log.Printf("fetch logs retry from=%d to=%d attempt=%d err=%v", fromBlock, toBlock, attempt, err)
		if waitErr := sleepWithContext(ctx, delay); waitErr != nil {
			return nil, waitErr
		}
	}
}

func (i *Indexer) fetchLogs(ctx context.Context, fromBlock, toBlock uint64) ([]types.Log, error) {
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(fromBlock)),
		ToBlock:   big.NewInt(int64(toBlock)),
		Addresses: []common.Address{i.contract},
		Topics:    [][]common.Hash{{i.decoder.EventTopics()[0], i.decoder.EventTopics()[1], i.decoder.EventTopics()[2], i.decoder.EventTopics()[3]}},
	}
	logs, err := i.client.FilterLogs(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("filter logs: %w", err)
	}
	return logs, nil
}

func (i *Indexer) retryBlockNumber(ctx context.Context) (uint64, error) {
	var attempt int
	for {
		blockNumber, err := i.client.BlockNumber(ctx)
		if err == nil {
			return blockNumber, nil
		}
		attempt++
		delay := backoffDuration(attempt)
		log.Printf("block number retry attempt=%d err=%v", attempt, err)
		if waitErr := sleepWithContext(ctx, delay); waitErr != nil {
			return 0, waitErr
		}
	}
}

func (i *Indexer) getBlockTime(ctx context.Context, blockNumber uint64, cache map[uint64]time.Time) (time.Time, error) {
	if ts, ok := cache[blockNumber]; ok {
		return ts, nil
	}

	header, err := i.client.HeaderByNumber(ctx, big.NewInt(int64(blockNumber)))
	if err != nil {
		return time.Time{}, fmt.Errorf("get header block %d: %w", blockNumber, err)
	}
	ts := time.Unix(int64(header.Time), 0).UTC()
	cache[blockNumber] = ts
	return ts, nil
}

func (i *Indexer) getNextFrom() uint64 {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.nextFrom
}

func (i *Indexer) setNextFrom(next uint64) {
	i.mu.Lock()
	i.nextFrom = next
	i.mu.Unlock()
}

func backoffDuration(attempt int) time.Duration {
	if attempt <= 0 {
		attempt = 1
	}
	seconds := math.Pow(2, float64(attempt-1))
	if seconds > 30 {
		seconds = 30
	}
	return time.Duration(seconds) * time.Second
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func minUint64(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

type decodeError struct {
	cause error
}

func (e *decodeError) Error() string {
	return fmt.Sprintf("decode event: %v", e.cause)
}
