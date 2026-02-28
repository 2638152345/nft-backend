CREATE TABLE IF NOT EXISTS auctions (
  id BIGINT UNSIGNED PRIMARY KEY,
  nft_address VARCHAR(42) NOT NULL,
  token_id VARCHAR(78) NOT NULL,
  seller VARCHAR(42) NOT NULL,
  bid_token VARCHAR(42) NOT NULL,
  start_price DECIMAL(65, 0) NOT NULL,
  highest_bid DECIMAL(65, 0) NOT NULL,
  highest_bidder VARCHAR(42) NULL,
  start_time DATETIME NOT NULL,
  end_time DATETIME NOT NULL,
  status ENUM('active', 'ended', 'cancelled') NOT NULL,
  created_block BIGINT UNSIGNED NOT NULL,
  updated_block BIGINT UNSIGNED NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  INDEX idx_auctions_status_end_time (status, end_time),
  INDEX idx_auctions_seller (seller),
  INDEX idx_auctions_nft_token (nft_address, token_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS auction_bids (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  auction_id BIGINT UNSIGNED NOT NULL,
  bidder VARCHAR(42) NOT NULL,
  amount DECIMAL(65, 0) NOT NULL,
  tx_hash VARCHAR(66) NOT NULL,
  log_index INT UNSIGNED NOT NULL,
  block_number BIGINT UNSIGNED NOT NULL,
  block_time DATETIME NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY uniq_bid_tx_log (tx_hash, log_index),
  INDEX idx_bids_auction_block (auction_id, block_number)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS processed_logs (
  tx_hash VARCHAR(66) NOT NULL,
  log_index INT UNSIGNED NOT NULL,
  event_name VARCHAR(32) NOT NULL,
  block_number BIGINT UNSIGNED NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (tx_hash, log_index)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS sync_state (
  id VARCHAR(64) PRIMARY KEY,
  last_processed_block BIGINT UNSIGNED NOT NULL,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS wallet_nft_cache (
  wallet VARCHAR(42) NOT NULL,
  chain VARCHAR(16) NOT NULL,
  payload_json LONGTEXT NOT NULL,
  fetched_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (wallet, chain)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
