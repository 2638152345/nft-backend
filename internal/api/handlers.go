package api

import (
	"net/http"
	"strconv"

	"nft-auction-homework/backend/internal/service"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *service.Service
}

func NewHandler(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) ListAuctions(c *gin.Context) {
	page := parseInt(c.DefaultQuery("page", "1"), 1)
	size := parseInt(c.DefaultQuery("size", "20"), 20)

	out, err := h.svc.ListAuctions(c.Request.Context(), service.ListAuctionsInput{
		Page:       page,
		Size:       size,
		SortBy:     c.DefaultQuery("sort_by", "created_time"),
		Order:      c.DefaultQuery("order", "desc"),
		Status:     c.Query("status"),
		Seller:     c.Query("seller"),
		NFTAddress: c.Query("nft_address"),
	})
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	respondOK(c, out)
}

func (h *Handler) ListAuctionBids(c *gin.Context) {
	auctionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid auction id")
		return
	}

	page := parseInt(c.DefaultQuery("page", "1"), 1)
	size := parseInt(c.DefaultQuery("size", "20"), 20)

	out, err := h.svc.ListAuctionBids(c.Request.Context(), service.ListBidsInput{
		AuctionID: auctionID,
		Page:      page,
		Size:      size,
		Order:     c.DefaultQuery("order", "desc"),
	})
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	respondOK(c, out)
}

func (h *Handler) GetStats(c *gin.Context) {
	stats, err := h.svc.GetStats(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	respondOK(c, stats)
}

func (h *Handler) GetWalletNFTs(c *gin.Context) {
	wallet := c.Param("address")
	page := parseInt(c.DefaultQuery("page", "1"), 1)
	size := parseInt(c.DefaultQuery("size", "20"), 20)

	out, err := h.svc.GetWalletNFTs(c.Request.Context(), wallet, page, size)
	if err != nil {
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}
	respondOK(c, out)
}

func (h *Handler) GetSyncStatus(c *gin.Context) {
	status, err := h.svc.GetSyncStatus(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	respondOK(c, status)
}

func respondOK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "ok",
		"data": data,
	})
}

func respondError(c *gin.Context, statusCode int, msg string) {
	c.JSON(statusCode, gin.H{
		"code": statusCode,
		"msg":  msg,
	})
}

func parseInt(raw string, fallback int) int {
	val, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return val
}
