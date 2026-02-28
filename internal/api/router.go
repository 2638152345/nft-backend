package api

import "github.com/gin-gonic/gin"

func NewRouter(handler *Handler) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger())

	v1 := r.Group("/api/v1")
	{
		v1.GET("/auctions", handler.ListAuctions)
		v1.GET("/auctions/:id/bids", handler.ListAuctionBids)
		v1.GET("/stats", handler.GetStats)
		v1.GET("/wallets/:address/nfts", handler.GetWalletNFTs)
		v1.GET("/sync/status", handler.GetSyncStatus)
	}

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	return r
}
