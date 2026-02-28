package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"nft-auction-homework/backend/internal/alchemy"
	"nft-auction-homework/backend/internal/config"
	"nft-auction-homework/backend/internal/repository"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

type Service struct {
	repo          *repository.Repository
	alchemyClient *alchemy.Client
	ethClient     *ethclient.Client
	cfg           config.Config
}

type ListAuctionsInput struct {
	Page       int
	Size       int
	SortBy     string
	Order      string
	Status     string
	Seller     string
	NFTAddress string
}

type ListBidsInput struct {
	AuctionID uint64
	Page      int
	Size      int
	Order     string
}

type Pagination struct {
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	Size       int   `json:"size"`
	TotalPages int   `json:"total_pages"`
}

type ListAuctionsOutput struct {
	Items      any        `json:"items"`
	Pagination Pagination `json:"pagination"`
}

type ListBidsOutput struct {
	Items      any        `json:"items"`
	Pagination Pagination `json:"pagination"`
}

type WalletNFTOutput struct {
	Wallet string         `json:"wallet"`
	Chain  string         `json:"chain"`
	Page   int            `json:"page"`
	Size   int            `json:"size"`
	Source string         `json:"source"`
	Data   map[string]any `json:"data"`
}

type SyncStatusOutput struct {
	SyncStateID        string `json:"sync_state_id"`
	LastProcessedBlock uint64 `json:"last_processed_block"`
	SafeHead           uint64 `json:"safe_head"`
	Lag                uint64 `json:"lag"`
	ContractAddress    string `json:"contract_address"`
	DeployBlock        uint64 `json:"deploy_block"`
	Confirmations      int64  `json:"confirmations"`
}

func New(repo *repository.Repository, alchemyClient *alchemy.Client, ethClient *ethclient.Client, cfg config.Config) *Service {
	return &Service{
		repo:          repo,
		alchemyClient: alchemyClient,
		ethClient:     ethClient,
		cfg:           cfg,
	}
}

func (s *Service) ListAuctions(ctx context.Context, in ListAuctionsInput) (ListAuctionsOutput, error) {
	page, size := normalizePageSize(in.Page, in.Size, 20, 100)

	sortField := "start_time"
	switch strings.ToLower(in.SortBy) {
	case "", "created_time":
		sortField = "start_time"
	case "highest_bid":
		sortField = "highest_bid"
	case "end_time":
		sortField = "end_time"
	default:
		return ListAuctionsOutput{}, fmt.Errorf("invalid sort_by")
	}

	sortOrder := normalizeOrder(in.Order)
	status := strings.ToLower(strings.TrimSpace(in.Status))
	if status != "" && status != "active" && status != "ended" && status != "cancelled" {
		return ListAuctionsOutput{}, fmt.Errorf("invalid status")
	}

	q := repository.AuctionListQuery{
		Page:       page,
		Size:       size,
		SortField:  sortField,
		SortOrder:  sortOrder,
		Status:     status,
		Seller:     strings.ToLower(strings.TrimSpace(in.Seller)),
		NFTAddress: strings.ToLower(strings.TrimSpace(in.NFTAddress)),
	}

	items, total, err := s.repo.ListAuctions(ctx, q)
	if err != nil {
		return ListAuctionsOutput{}, err
	}

	return ListAuctionsOutput{
		Items: items,
		Pagination: Pagination{
			Total:      total,
			Page:       page,
			Size:       size,
			TotalPages: totalPages(total, size),
		},
	}, nil
}

func (s *Service) ListAuctionBids(ctx context.Context, in ListBidsInput) (ListBidsOutput, error) {
	page, size := normalizePageSize(in.Page, in.Size, 20, 100)
	order := normalizeOrder(in.Order)

	items, total, err := s.repo.ListAuctionBids(ctx, repository.BidListQuery{
		AuctionID: in.AuctionID,
		Page:      page,
		Size:      size,
		SortOrder: order,
	})
	if err != nil {
		return ListBidsOutput{}, err
	}

	return ListBidsOutput{
		Items: items,
		Pagination: Pagination{
			Total:      total,
			Page:       page,
			Size:       size,
			TotalPages: totalPages(total, size),
		},
	}, nil
}

func (s *Service) GetStats(ctx context.Context) (repository.Stats, error) {
	return s.repo.GetStats(ctx)
}

func (s *Service) GetWalletNFTs(ctx context.Context, wallet string, page, size int) (WalletNFTOutput, error) {
	wallet = strings.ToLower(strings.TrimSpace(wallet))
	if !common.IsHexAddress(wallet) {
		return WalletNFTOutput{}, fmt.Errorf("invalid wallet address")
	}
	_, size = normalizePageSize(1, size, s.cfg.WalletDefaultSize, s.cfg.WalletMaxPageSize)
	if page < 1 {
		page = 1
	}

	chain := "sepolia"
	if page == 1 {
		cache, err := s.repo.GetWalletCache(ctx, wallet, chain)
		if err != nil {
			return WalletNFTOutput{}, err
		}
		if cache != nil && time.Since(cache.FetchedAt) <= s.cfg.WalletCacheTTL {
			data, err := decodeJSONMap(cache.PayloadJSON)
			if err != nil {
				return WalletNFTOutput{}, err
			}
			return WalletNFTOutput{
				Wallet: wallet,
				Chain:  chain,
				Page:   page,
				Size:   size,
				Source: "cache",
				Data:   data,
			}, nil
		}
	}

	raw, err := s.alchemyClient.FetchOwnerNFTsRaw(ctx, wallet, size, page)
	if err != nil {
		return WalletNFTOutput{}, err
	}

	if page == 1 {
		if err := s.repo.UpsertWalletCache(ctx, wallet, chain, string(raw), time.Now()); err != nil {
			return WalletNFTOutput{}, err
		}
	}

	data, err := decodeJSONMap(string(raw))
	if err != nil {
		return WalletNFTOutput{}, err
	}

	return WalletNFTOutput{
		Wallet: wallet,
		Chain:  chain,
		Page:   page,
		Size:   size,
		Source: "alchemy",
		Data:   data,
	}, nil
}

func (s *Service) GetSyncStatus(ctx context.Context) (SyncStatusOutput, error) {
	state, err := s.repo.GetSyncState(ctx, s.cfg.SyncStateID)
	if err != nil {
		return SyncStatusOutput{}, err
	}
	lastProcessed := uint64(0)
	if state != nil {
		lastProcessed = state.LastProcessedBlock
	}

	latest, err := s.ethClient.BlockNumber(ctx)
	if err != nil {
		return SyncStatusOutput{}, err
	}

	safeHead := uint64(0)
	if int64(latest) > s.cfg.Confirmations {
		safeHead = latest - uint64(s.cfg.Confirmations)
	}

	lag := uint64(0)
	if safeHead > lastProcessed {
		lag = safeHead - lastProcessed
	}

	return SyncStatusOutput{
		SyncStateID:        s.cfg.SyncStateID,
		LastProcessedBlock: lastProcessed,
		SafeHead:           safeHead,
		Lag:                lag,
		ContractAddress:    strings.ToLower(s.cfg.AuctionContractAddress),
		DeployBlock:        s.cfg.AuctionDeployBlock,
		Confirmations:      s.cfg.Confirmations,
	}, nil
}

func normalizePageSize(page, size, defaultSize, maxSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size <= 0 {
		size = defaultSize
	}
	if size > maxSize {
		size = maxSize
	}
	return page, size
}

func normalizeOrder(order string) string {
	if strings.ToLower(strings.TrimSpace(order)) == "asc" {
		return "asc"
	}
	return "desc"
}

func totalPages(total int64, size int) int {
	if size <= 0 {
		return 0
	}
	return int(math.Ceil(float64(total) / float64(size)))
}

func decodeJSONMap(raw string) (map[string]any, error) {
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("decode json map: %w", err)
	}
	return out, nil
}
