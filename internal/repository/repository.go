package repository

import (
	"context"
	"fmt"
	"time"

	"nft-auction-homework/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

type AuctionListQuery struct {
	Page       int
	Size       int
	SortField  string
	SortOrder  string
	Status     string
	Seller     string
	NFTAddress string
}

type BidListQuery struct {
	AuctionID uint64
	Page      int
	Size      int
	SortOrder string
}

type Stats struct {
	TotalAuctions int64 `json:"total_auctions"`
	TotalBids     int64 `json:"total_bids"`
}

func (r *Repository) WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return r.db.WithContext(ctx).Transaction(fn)
}

func (r *Repository) IsLogProcessedTx(tx *gorm.DB, txHash string, logIndex uint) (bool, error) {
	var count int64
	err := tx.Model(&model.ProcessedLog{}).
		Where("tx_hash = ? AND log_index = ?", txHash, logIndex).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Repository) InsertProcessedLogTx(tx *gorm.DB, log model.ProcessedLog) error {
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&log).Error
}

func (r *Repository) UpsertAuctionTx(tx *gorm.DB, auction model.Auction) error {
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"nft_address",
			"token_id",
			"seller",
			"bid_token",
			"start_price",
			"highest_bid",
			"highest_bidder",
			"start_time",
			"end_time",
			"status",
			"created_block",
			"updated_block",
			"updated_at",
		}),
	}).Create(&auction).Error
}

func (r *Repository) InsertBidTx(tx *gorm.DB, bid model.AuctionBid) error {
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&bid).Error
}

func (r *Repository) UpdateAuctionHighestBidTx(tx *gorm.DB, auctionID uint64, highestBid, highestBidder string, updatedBlock uint64) error {
	res := tx.Model(&model.Auction{}).
		Where("id = ?", auctionID).
		Updates(map[string]any{
			"highest_bid":    highestBid,
			"highest_bidder": highestBidder,
			"updated_block":  updatedBlock,
			"updated_at":     time.Now(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("auction %d not found", auctionID)
	}
	return nil
}

func (r *Repository) UpdateAuctionStatusTx(tx *gorm.DB, auctionID uint64, status string, updatedBlock uint64) error {
	res := tx.Model(&model.Auction{}).
		Where("id = ?", auctionID).
		Updates(map[string]any{
			"status":        status,
			"updated_block": updatedBlock,
			"updated_at":    time.Now(),
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("auction %d not found", auctionID)
	}
	return nil
}

func (r *Repository) GetSyncState(ctx context.Context, id string) (*model.SyncState, error) {
	var state model.SyncState
	err := r.db.WithContext(ctx).Where("id = ?", id).Take(&state).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &state, nil
}

func (r *Repository) UpsertSyncState(ctx context.Context, id string, lastProcessedBlock uint64) error {
	state := model.SyncState{ID: id, LastProcessedBlock: lastProcessedBlock, UpdatedAt: time.Now()}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"last_processed_block", "updated_at"}),
	}).Create(&state).Error
}

func (r *Repository) ListAuctions(ctx context.Context, q AuctionListQuery) ([]model.Auction, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.Auction{})

	if q.Status != "" {
		query = query.Where("status = ?", q.Status)
	}
	if q.Seller != "" {
		query = query.Where("seller = ?", q.Seller)
	}
	if q.NFTAddress != "" {
		query = query.Where("nft_address = ?", q.NFTAddress)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (q.Page - 1) * q.Size
	var items []model.Auction
	err := query.Order(fmt.Sprintf("%s %s", q.SortField, q.SortOrder)).
		Limit(q.Size).
		Offset(offset).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r *Repository) ListAuctionBids(ctx context.Context, q BidListQuery) ([]model.AuctionBid, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.AuctionBid{}).Where("auction_id = ?", q.AuctionID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (q.Page - 1) * q.Size
	var bids []model.AuctionBid
	err := query.Order(fmt.Sprintf("block_number %s, log_index %s", q.SortOrder, q.SortOrder)).
		Limit(q.Size).
		Offset(offset).
		Find(&bids).Error
	if err != nil {
		return nil, 0, err
	}

	return bids, total, nil
}

func (r *Repository) GetStats(ctx context.Context) (Stats, error) {
	var stats Stats
	if err := r.db.WithContext(ctx).Model(&model.Auction{}).Count(&stats.TotalAuctions).Error; err != nil {
		return Stats{}, err
	}
	if err := r.db.WithContext(ctx).Model(&model.AuctionBid{}).Count(&stats.TotalBids).Error; err != nil {
		return Stats{}, err
	}
	return stats, nil
}

func (r *Repository) GetWalletCache(ctx context.Context, wallet, chain string) (*model.WalletNFTCache, error) {
	var cache model.WalletNFTCache
	err := r.db.WithContext(ctx).
		Where("wallet = ? AND chain = ?", wallet, chain).
		Take(&cache).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &cache, nil
}

func (r *Repository) UpsertWalletCache(ctx context.Context, wallet, chain, payload string, fetchedAt time.Time) error {
	row := model.WalletNFTCache{
		Wallet:      wallet,
		Chain:       chain,
		PayloadJSON: payload,
		FetchedAt:   fetchedAt,
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "wallet"}, {Name: "chain"}},
		DoUpdates: clause.AssignmentColumns([]string{"payload_json", "fetched_at", "updated_at"}),
	}).Create(&row).Error
}
