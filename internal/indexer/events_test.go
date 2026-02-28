package indexer

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestDecodeAuctionCreated(t *testing.T) {
	decoder, err := NewDecoder()
	if err != nil {
		t.Fatalf("new decoder: %v", err)
	}

	parsed, err := abi.JSON(strings.NewReader(eventABI))
	if err != nil {
		t.Fatalf("parse abi: %v", err)
	}
	evt := parsed.Events["AuctionCreated"]

	nft := common.HexToAddress("0x1111111111111111111111111111111111111111")
	seller := common.HexToAddress("0x2222222222222222222222222222222222222222")
	bidToken := common.HexToAddress("0x3333333333333333333333333333333333333333")

	data, err := evt.Inputs.NonIndexed().Pack(
		big.NewInt(123),
		bidToken,
		big.NewInt(1000),
		big.NewInt(1700000000),
	)
	if err != nil {
		t.Fatalf("pack data: %v", err)
	}

	log := types.Log{
		Topics: []common.Hash{
			evt.ID,
			common.BigToHash(big.NewInt(1)),
			addressTopic(nft),
			addressTopic(seller),
		},
		Data: data,
	}

	decoded, eventName, err := decoder.Decode(log)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if eventName != "AuctionCreated" {
		t.Fatalf("unexpected event name: %s", eventName)
	}

	created, ok := decoded.(AuctionCreatedEvent)
	if !ok {
		t.Fatalf("decoded type mismatch")
	}
	if created.AuctionID != 1 || created.TokenID != "123" || created.StartPrice != "1000" {
		t.Fatalf("unexpected created event: %+v", created)
	}
}

func TestDecodeAuctionBidEndedCancelled(t *testing.T) {
	decoder, err := NewDecoder()
	if err != nil {
		t.Fatalf("new decoder: %v", err)
	}
	parsed, err := abi.JSON(strings.NewReader(eventABI))
	if err != nil {
		t.Fatalf("parse abi: %v", err)
	}

	bidder := common.HexToAddress("0x4444444444444444444444444444444444444444")
	winner := common.HexToAddress("0x5555555555555555555555555555555555555555")

	bidEvt := parsed.Events["AuctionBid"]
	bidData, _ := bidEvt.Inputs.NonIndexed().Pack(big.NewInt(2000))
	bidLog := types.Log{Topics: []common.Hash{bidEvt.ID, common.BigToHash(big.NewInt(7)), addressTopic(bidder)}, Data: bidData}
	decodedBid, _, err := decoder.Decode(bidLog)
	if err != nil {
		t.Fatalf("decode bid: %v", err)
	}
	if decodedBid.(AuctionBidEvent).Amount != "2000" {
		t.Fatalf("unexpected bid payload")
	}

	endEvt := parsed.Events["AuctionEnded"]
	endData, _ := endEvt.Inputs.NonIndexed().Pack(big.NewInt(3000))
	endLog := types.Log{Topics: []common.Hash{endEvt.ID, common.BigToHash(big.NewInt(7)), addressTopic(winner)}, Data: endData}
	decodedEnd, _, err := decoder.Decode(endLog)
	if err != nil {
		t.Fatalf("decode ended: %v", err)
	}
	if decodedEnd.(AuctionEndedEvent).FinalPrice != "3000" {
		t.Fatalf("unexpected ended payload")
	}

	cancelEvt := parsed.Events["AuctionCancelled"]
	cancelLog := types.Log{Topics: []common.Hash{cancelEvt.ID, common.BigToHash(big.NewInt(7))}}
	decodedCancel, _, err := decoder.Decode(cancelLog)
	if err != nil {
		t.Fatalf("decode cancelled: %v", err)
	}
	if decodedCancel.(AuctionCancelledEvent).AuctionID != 7 {
		t.Fatalf("unexpected cancelled payload")
	}
}

func addressTopic(addr common.Address) common.Hash {
	return common.BytesToHash(common.LeftPadBytes(addr.Bytes(), 32))
}
