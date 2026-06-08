package model

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// Side represents the order side (BUY or SELL)
type Side int

const (
	Buy Side = iota
	Sell
)

func (s Side) String() string {
	switch s {
	case Buy:
		return "BUY"
	case Sell:
		return "SELL"
	default:
		return "UNKNOWN"
	}
}

// Status represents the order status
type Status string

const (
	StatusOpen      Status = "OPEN"
	StatusFilled    Status = "FILLED"
	StatusCancelled Status = "CANCELLED"
	StatusExpired   Status = "EXPIRED"
)

// OrderType represents the type of order
type OrderType string

const (
	Limit  OrderType = "LIMIT"
	Market OrderType = "MARKET"
)

// Order represents an order in the order book
type Order struct {
	ID            string          `json:"id"`
	Owner         string          `json:"owner"`
	MarketID      string          `json:"market_id"`
	AssetID       string          `json:"asset_id"`
	Side          Side            `json:"side"`
	Price         decimal.Decimal `json:"price"`
	Size          decimal.Decimal `json:"size"`
	RemainingSize decimal.Decimal `json:"remaining_size"`
	Type          OrderType       `json:"type"`
	Status        Status          `json:"status"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// NewOrder creates a new order
func NewOrder(id, owner, marketID, assetID string, side Side, price, size decimal.Decimal, orderType OrderType) *Order {
	now := time.Now()
	return &Order{
		ID:            id,
		Owner:         owner,
		MarketID:      marketID,
		AssetID:       assetID,
		Side:          side,
		Price:         price,
		Size:          size,
		RemainingSize: size,
		Type:          orderType,
		Status:        StatusOpen,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// IsBuy returns true if the order is a buy order
func (o *Order) IsBuy() bool {
	return o.Side == Buy
}

// IsSell returns true if the order is a sell order
func (o *Order) IsSell() bool {
	return o.Side == Sell
}

// IsOpen returns true if the order is still open
func (o *Order) IsOpen() bool {
	return o.Status == StatusOpen
}

// Remaining returns the remaining size of the order
func (o *Order) Remaining() decimal.Decimal {
	return o.RemainingSize
}

// Fill reduces the remaining size by the given amount
func (o *Order) Fill(amount decimal.Decimal) {
	if amount.GreaterThan(o.RemainingSize) {
		amount = o.RemainingSize
	}
	if amount.IsZero() {
		return
	}
	o.RemainingSize = o.RemainingSize.Sub(amount)
	if o.RemainingSize.IsZero() {
		o.Status = StatusFilled
	}
	o.UpdatedAt = time.Now()
}

// RedisSortedSetScore returns the score used for Redis sorted set storage.
// For buy orders: score = price * 1e18 + (1e18 - timestamp) to get best price first, then earlier timestamp
// For sell orders: score = price * 1e18 + timestamp to get lowest price first, then earlier timestamp
func (o *Order) RedisScore() float64 {
	// Scale price to avoid floating point issues (price * 1e18)
	priceScaled := o.Price.Mul(decimal.NewFromInt(1e18)).Truncate(0)
	base := priceScaled.IntPart()

	// Add timestamp to break ties (time-based priority)
	timestamp := o.CreatedAt.UnixNano()

	if o.IsBuy() {
		// Higher price = better, earlier timestamp = better
		// So: price * scale + (maxTimestamp - timestamp)
		return float64(base) + float64(9223372036854775807-timestamp)
	}
	// Sell: lower price = better, earlier timestamp = better
	return float64(base) + float64(timestamp)
}

// RedisScoreKey returns the key for the sorted set in Redis
func (o *Order) RedisScoreKey() string {
	return fmt.Sprintf("%s:%s:%s:%s:%d", o.MarketID, o.AssetID, o.Side, o.Price.String(), o.CreatedAt.UnixNano())
}
