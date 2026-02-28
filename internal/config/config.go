package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/joho/godotenv"
)

type Config struct {
	ServerPort string

	MySQLDSN      string
	MigrationsDir string

	ChainID                int64
	RPCHTTPURL             string
	AuctionContractAddress string
	AuctionDeployBlock     uint64
	SyncStateID            string

	Confirmations  int64
	BlockChunkSize uint64
	ReorgBuffer    uint64
	PollInterval   time.Duration

	AlchemyAPIKey     string
	AlchemyNetwork    string
	WalletCacheTTL    time.Duration
	WalletDefaultSize int
	WalletMaxPageSize int
}

func Load() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		ServerPort:             getEnv("SERVER_PORT", "8080"),
		MySQLDSN:               strings.TrimSpace(os.Getenv("MYSQL_DSN")),
		MigrationsDir:          getEnv("MIGRATIONS_DIR", "./migrations"),
		RPCHTTPURL:             strings.TrimSpace(os.Getenv("RPC_HTTP_URL")),
		AuctionContractAddress: strings.TrimSpace(os.Getenv("AUCTION_CONTRACT_ADDRESS")),
		SyncStateID:            getEnv("SYNC_STATE_ID", "nft_auction_sepolia_main"),
		AlchemyAPIKey:          strings.TrimSpace(os.Getenv("ALCHEMY_API_KEY")),
		AlchemyNetwork:         getEnv("ALCHEMY_NETWORK", "eth-sepolia"),
	}

	chainID, err := getInt64("CHAIN_ID", 11155111)
	if err != nil {
		return Config{}, err
	}
	cfg.ChainID = chainID

	deployBlock, err := getUint64Required("AUCTION_DEPLOY_BLOCK")
	if err != nil {
		return Config{}, err
	}
	cfg.AuctionDeployBlock = deployBlock

	confirmations, err := getInt64("CONFIRMATIONS", 12)
	if err != nil {
		return Config{}, err
	}
	cfg.Confirmations = confirmations

	chunkSize, err := getUint64("BLOCK_CHUNK_SIZE", 2000)
	if err != nil {
		return Config{}, err
	}
	cfg.BlockChunkSize = chunkSize

	reorgBuffer, err := getUint64("REORG_BUFFER", 20)
	if err != nil {
		return Config{}, err
	}
	cfg.ReorgBuffer = reorgBuffer

	pollIntervalSec, err := getInt64("POLL_INTERVAL_SEC", 8)
	if err != nil {
		return Config{}, err
	}
	cfg.PollInterval = time.Duration(pollIntervalSec) * time.Second

	cacheTTL, err := getInt64("WALLET_NFT_CACHE_TTL_SEC", 60)
	if err != nil {
		return Config{}, err
	}
	cfg.WalletCacheTTL = time.Duration(cacheTTL) * time.Second

	walletDefaultSize, err := getInt("WALLET_NFT_DEFAULT_SIZE", 20)
	if err != nil {
		return Config{}, err
	}
	cfg.WalletDefaultSize = walletDefaultSize

	walletMaxSize, err := getInt("WALLET_NFT_MAX_SIZE", 100)
	if err != nil {
		return Config{}, err
	}
	cfg.WalletMaxPageSize = walletMaxSize

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) validate() error {
	if c.MySQLDSN == "" {
		return fmt.Errorf("MYSQL_DSN is required")
	}
	if c.RPCHTTPURL == "" {
		return fmt.Errorf("RPC_HTTP_URL is required")
	}
	if !common.IsHexAddress(c.AuctionContractAddress) {
		return fmt.Errorf("AUCTION_CONTRACT_ADDRESS is invalid: %s", c.AuctionContractAddress)
	}
	if c.Confirmations < 0 {
		return fmt.Errorf("CONFIRMATIONS must be >= 0")
	}
	if c.BlockChunkSize == 0 {
		return fmt.Errorf("BLOCK_CHUNK_SIZE must be > 0")
	}
	if c.PollInterval <= 0 {
		return fmt.Errorf("POLL_INTERVAL_SEC must be > 0")
	}
	if c.AlchemyAPIKey == "" {
		return fmt.Errorf("ALCHEMY_API_KEY is required")
	}
	if c.WalletDefaultSize <= 0 {
		return fmt.Errorf("WALLET_NFT_DEFAULT_SIZE must be > 0")
	}
	if c.WalletMaxPageSize < c.WalletDefaultSize {
		return fmt.Errorf("WALLET_NFT_MAX_SIZE must be >= WALLET_NFT_DEFAULT_SIZE")
	}
	return nil
}

func getEnv(key, fallback string) string {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}
	return val
}

func getInt64(key string, fallback int64) (int64, error) {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback, nil
	}
	n, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be int64: %w", key, err)
	}
	return n, nil
}

func getUint64(key string, fallback uint64) (uint64, error) {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback, nil
	}
	n, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be uint64: %w", key, err)
	}
	return n, nil
}

func getUint64Required(key string) (uint64, error) {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return 0, fmt.Errorf("%s is required", key)
	}
	n, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be uint64: %w", key, err)
	}
	return n, nil
}

func getInt(key string, fallback int) (int, error) {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("%s must be int: %w", key, err)
	}
	return n, nil
}
