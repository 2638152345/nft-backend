package model

import "time"

const (
	AuctionStatusActive    = "active"
	AuctionStatusEnded     = "ended"
	AuctionStatusCancelled = "cancelled"
)

type Auction struct {
	ID            uint64    `gorm:"primaryKey;column:id" json:"id"`
	NFTAddress    string    `gorm:"type:varchar(42);not null;index:idx_auctions_nft_token,priority:1" json:"nft_address"`
	TokenID       string    `gorm:"type:varchar(78);not null;index:idx_auctions_nft_token,priority:2" json:"token_id"`
	Seller        string    `gorm:"type:varchar(42);not null;index:idx_auctions_seller" json:"seller"`
	BidToken      string    `gorm:"type:varchar(42);not null" json:"bid_token"`
	StartPrice    string    `gorm:"type:decimal(65,0);not null" json:"start_price"`
	HighestBid    string    `gorm:"type:decimal(65,0);not null" json:"highest_bid"`
	HighestBidder *string   `gorm:"type:varchar(42)" json:"highest_bidder"`
	StartTime     time.Time `gorm:"type:datetime;not null" json:"start_time"`
	EndTime       time.Time `gorm:"type:datetime;not null;index:idx_auctions_status_end_time,priority:2" json:"end_time"`
	Status        string    `gorm:"type:enum('active','ended','cancelled');not null;index:idx_auctions_status_end_time,priority:1" json:"status"`
	CreatedBlock  uint64    `gorm:"not null" json:"created_block"`
	UpdatedBlock  uint64    `gorm:"not null" json:"updated_block"`
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Auction) TableName() string { return "auctions" }

type AuctionBid struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	AuctionID   uint64    `gorm:"not null;index:idx_bids_auction_block,priority:1" json:"auction_id"`
	Bidder      string    `gorm:"type:varchar(42);not null" json:"bidder"`
	Amount      string    `gorm:"type:decimal(65,0);not null" json:"amount"`
	TxHash      string    `gorm:"type:varchar(66);not null;uniqueIndex:uniq_bid_tx_log,priority:1" json:"tx_hash"`
	LogIndex    uint      `gorm:"not null;uniqueIndex:uniq_bid_tx_log,priority:2" json:"log_index"`
	BlockNumber uint64    `gorm:"not null;index:idx_bids_auction_block,priority:2" json:"block_number"`
	BlockTime   time.Time `gorm:"type:datetime;not null" json:"block_time"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (AuctionBid) TableName() string { return "auction_bids" }

type ProcessedLog struct {
	TxHash      string    `gorm:"type:varchar(66);primaryKey;column:tx_hash"`
	LogIndex    uint      `gorm:"primaryKey;column:log_index"`
	EventName   string    `gorm:"type:varchar(32);not null"`
	BlockNumber uint64    `gorm:"not null"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
}

func (ProcessedLog) TableName() string { return "processed_logs" }

type SyncState struct {
	ID                 string    `gorm:"type:varchar(64);primaryKey;column:id" json:"id"`
	LastProcessedBlock uint64    `gorm:"not null;column:last_processed_block" json:"last_processed_block"`
	UpdatedAt          time.Time `gorm:"autoUpdateTime;column:updated_at" json:"updated_at"`
}

func (SyncState) TableName() string { return "sync_state" }

type WalletNFTCache struct {
	Wallet      string    `gorm:"type:varchar(42);primaryKey;column:wallet"`
	Chain       string    `gorm:"type:varchar(16);primaryKey;column:chain"`
	PayloadJSON string    `gorm:"type:longtext;not null;column:payload_json"`
	FetchedAt   time.Time `gorm:"type:datetime;not null;column:fetched_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

func (WalletNFTCache) TableName() string { return "wallet_nft_cache" }
