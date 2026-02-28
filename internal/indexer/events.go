package indexer

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

const eventABI = `[
	{"anonymous":false,"inputs":[
		{"indexed":true,"internalType":"uint256","name":"auctionId","type":"uint256"},
		{"indexed":true,"internalType":"address","name":"nftAddress","type":"address"},
		{"indexed":true,"internalType":"address","name":"seller","type":"address"},
		{"indexed":false,"internalType":"uint256","name":"tokenId","type":"uint256"},
		{"indexed":false,"internalType":"address","name":"bidToken","type":"address"},
		{"indexed":false,"internalType":"uint256","name":"startPrice","type":"uint256"},
		{"indexed":false,"internalType":"uint256","name":"endTime","type":"uint256"}
	],"name":"AuctionCreated","type":"event"},
	{"anonymous":false,"inputs":[
		{"indexed":true,"internalType":"uint256","name":"auctionId","type":"uint256"},
		{"indexed":true,"internalType":"address","name":"bidder","type":"address"},
		{"indexed":false,"internalType":"uint256","name":"amount","type":"uint256"}
	],"name":"AuctionBid","type":"event"},
	{"anonymous":false,"inputs":[
		{"indexed":true,"internalType":"uint256","name":"auctionId","type":"uint256"},
		{"indexed":true,"internalType":"address","name":"winner","type":"address"},
		{"indexed":false,"internalType":"uint256","name":"finalPrice","type":"uint256"}
	],"name":"AuctionEnded","type":"event"},
	{"anonymous":false,"inputs":[
		{"indexed":true,"internalType":"uint256","name":"auctionId","type":"uint256"}
	],"name":"AuctionCancelled","type":"event"}
]`

type Decoder struct {
	abi         abi.ABI
	createdID   common.Hash
	bidID       common.Hash
	endedID     common.Hash
	cancelledID common.Hash
}

type AuctionCreatedEvent struct {
	AuctionID  uint64
	NFTAddress string
	Seller     string
	TokenID    string
	BidToken   string
	StartPrice string
	EndTime    uint64
}

type AuctionBidEvent struct {
	AuctionID uint64
	Bidder    string
	Amount    string
}

type AuctionEndedEvent struct {
	AuctionID  uint64
	Winner     string
	FinalPrice string
}

type AuctionCancelledEvent struct {
	AuctionID uint64
}

func NewDecoder() (*Decoder, error) {
	parsed, err := abi.JSON(strings.NewReader(eventABI))
	if err != nil {
		return nil, fmt.Errorf("parse event abi: %w", err)
	}

	return &Decoder{
		abi:         parsed,
		createdID:   parsed.Events["AuctionCreated"].ID,
		bidID:       parsed.Events["AuctionBid"].ID,
		endedID:     parsed.Events["AuctionEnded"].ID,
		cancelledID: parsed.Events["AuctionCancelled"].ID,
	}, nil
}

func (d *Decoder) EventTopics() []common.Hash {
	return []common.Hash{d.createdID, d.bidID, d.endedID, d.cancelledID}
}

func (d *Decoder) Decode(log types.Log) (any, string, error) {
	if len(log.Topics) == 0 {
		return nil, "", fmt.Errorf("log has no topics")
	}
	switch log.Topics[0] {
	case d.createdID:
		evt, err := d.decodeAuctionCreated(log)
		return evt, "AuctionCreated", err
	case d.bidID:
		evt, err := d.decodeAuctionBid(log)
		return evt, "AuctionBid", err
	case d.endedID:
		evt, err := d.decodeAuctionEnded(log)
		return evt, "AuctionEnded", err
	case d.cancelledID:
		evt, err := d.decodeAuctionCancelled(log)
		return evt, "AuctionCancelled", err
	default:
		return nil, "", fmt.Errorf("unsupported event topic: %s", log.Topics[0].Hex())
	}
}

func (d *Decoder) decodeAuctionCreated(log types.Log) (AuctionCreatedEvent, error) {
	if len(log.Topics) < 4 {
		return AuctionCreatedEvent{}, fmt.Errorf("AuctionCreated topics length invalid")
	}
	event := d.abi.Events["AuctionCreated"]
	values, err := event.Inputs.NonIndexed().Unpack(log.Data)
	if err != nil {
		return AuctionCreatedEvent{}, fmt.Errorf("unpack AuctionCreated: %w", err)
	}
	if len(values) != 4 {
		return AuctionCreatedEvent{}, fmt.Errorf("AuctionCreated values length invalid")
	}

	tokenID, ok := values[0].(*big.Int)
	if !ok {
		return AuctionCreatedEvent{}, fmt.Errorf("AuctionCreated tokenId type invalid")
	}
	bidToken, ok := values[1].(common.Address)
	if !ok {
		return AuctionCreatedEvent{}, fmt.Errorf("AuctionCreated bidToken type invalid")
	}
	startPrice, ok := values[2].(*big.Int)
	if !ok {
		return AuctionCreatedEvent{}, fmt.Errorf("AuctionCreated startPrice type invalid")
	}
	endTime, ok := values[3].(*big.Int)
	if !ok {
		return AuctionCreatedEvent{}, fmt.Errorf("AuctionCreated endTime type invalid")
	}

	return AuctionCreatedEvent{
		AuctionID:  topicToUint64(log.Topics[1]),
		NFTAddress: strings.ToLower(topicToAddress(log.Topics[2]).Hex()),
		Seller:     strings.ToLower(topicToAddress(log.Topics[3]).Hex()),
		TokenID:    tokenID.String(),
		BidToken:   strings.ToLower(bidToken.Hex()),
		StartPrice: startPrice.String(),
		EndTime:    endTime.Uint64(),
	}, nil
}

func (d *Decoder) decodeAuctionBid(log types.Log) (AuctionBidEvent, error) {
	if len(log.Topics) < 3 {
		return AuctionBidEvent{}, fmt.Errorf("AuctionBid topics length invalid")
	}
	event := d.abi.Events["AuctionBid"]
	values, err := event.Inputs.NonIndexed().Unpack(log.Data)
	if err != nil {
		return AuctionBidEvent{}, fmt.Errorf("unpack AuctionBid: %w", err)
	}
	if len(values) != 1 {
		return AuctionBidEvent{}, fmt.Errorf("AuctionBid values length invalid")
	}
	amount, ok := values[0].(*big.Int)
	if !ok {
		return AuctionBidEvent{}, fmt.Errorf("AuctionBid amount type invalid")
	}

	return AuctionBidEvent{
		AuctionID: topicToUint64(log.Topics[1]),
		Bidder:    strings.ToLower(topicToAddress(log.Topics[2]).Hex()),
		Amount:    amount.String(),
	}, nil
}

func (d *Decoder) decodeAuctionEnded(log types.Log) (AuctionEndedEvent, error) {
	if len(log.Topics) < 3 {
		return AuctionEndedEvent{}, fmt.Errorf("AuctionEnded topics length invalid")
	}
	event := d.abi.Events["AuctionEnded"]
	values, err := event.Inputs.NonIndexed().Unpack(log.Data)
	if err != nil {
		return AuctionEndedEvent{}, fmt.Errorf("unpack AuctionEnded: %w", err)
	}
	if len(values) != 1 {
		return AuctionEndedEvent{}, fmt.Errorf("AuctionEnded values length invalid")
	}
	finalPrice, ok := values[0].(*big.Int)
	if !ok {
		return AuctionEndedEvent{}, fmt.Errorf("AuctionEnded finalPrice type invalid")
	}

	return AuctionEndedEvent{
		AuctionID:  topicToUint64(log.Topics[1]),
		Winner:     strings.ToLower(topicToAddress(log.Topics[2]).Hex()),
		FinalPrice: finalPrice.String(),
	}, nil
}

func (d *Decoder) decodeAuctionCancelled(log types.Log) (AuctionCancelledEvent, error) {
	if len(log.Topics) < 2 {
		return AuctionCancelledEvent{}, fmt.Errorf("AuctionCancelled topics length invalid")
	}
	return AuctionCancelledEvent{
		AuctionID: topicToUint64(log.Topics[1]),
	}, nil
}

func topicToUint64(topic common.Hash) uint64 {
	return new(big.Int).SetBytes(topic.Bytes()).Uint64()
}

func topicToAddress(topic common.Hash) common.Address {
	return common.BytesToAddress(topic.Bytes()[12:])
}
