package service

import (
	"context"
	"testing"
)

func TestListAuctionsValidation(t *testing.T) {
	svc := &Service{}

	_, err := svc.ListAuctions(context.Background(), ListAuctionsInput{SortBy: "bad"})
	if err == nil {
		t.Fatalf("expected error for invalid sort")
	}

	_, err = svc.ListAuctions(context.Background(), ListAuctionsInput{Status: "bad"})
	if err == nil {
		t.Fatalf("expected error for invalid status")
	}
}

func TestNormalizePageSizeAndOrder(t *testing.T) {
	page, size := normalizePageSize(-1, 999, 20, 100)
	if page != 1 || size != 100 {
		t.Fatalf("unexpected normalized page/size: %d %d", page, size)
	}

	if normalizeOrder("asc") != "asc" {
		t.Fatalf("expected asc")
	}
	if normalizeOrder("xxx") != "desc" {
		t.Fatalf("expected desc fallback")
	}
}

func TestTotalPages(t *testing.T) {
	if totalPages(0, 20) != 0 {
		t.Fatalf("expected 0")
	}
	if totalPages(21, 20) != 2 {
		t.Fatalf("expected 2")
	}
}
