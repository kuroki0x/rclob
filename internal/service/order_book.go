package service

import (
	"context"
	"fmt"
	"sync"

	"github.com/kuroki0x/rclob/internal/model"
	"github.com/kuroki0x/rclob/internal/repository"
	"github.com/shopspring/decimal"
)

// OrderBookService handles order book operations and matching
type OrderBookService struct {
	repo *repository.OrderBookRepository
	mu   sync.RWMutex
}

// NewOrderBookService creates a new order book service
func NewOrderBookService(repo *repository.OrderBookRepository) *OrderBookService {
	return &OrderBookService{
		repo: repo,
	}
}

// CreateOrder adds a new order to the order book and attempts matching
func (s *OrderBookService) CreateOrder(ctx context.Context, order *model.Order) (*CreateOrderResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate the order
	if err := s.validateOrder(order); err != nil {
		return nil, fmt.Errorf("invalid order: %w", err)
	}

	// Attempt to match the order with existing orders
	matches, err := s.matchOrder(ctx, order)
	if err != nil {
		return nil, fmt.Errorf("matching failed: %w", err)
	}

	// Apply matches
	remainingSize := order.RemainingSize
	for _, match := range matches {
		matchSize := min(match.MakerOrder.RemainingSize, order.RemainingSize)
		match.MatchedSize = matchSize

		// Fill both orders
		match.MakerOrder.Fill(matchSize)
		order.Fill(matchSize)
		remainingSize = remainingSize.Sub(matchSize)

		// Remove filled maker order from book
		if match.MakerOrder.IsOpen() {
			if err := s.repo.UpdateOrder(ctx, match.MakerOrder); err != nil {
				return nil, fmt.Errorf("update maker order: %w", err)
			}
		} else {
			if err := s.repo.RemoveOrderFromSide(ctx, match.MakerOrder); err != nil {
				return nil, fmt.Errorf("remove filled maker order: %w", err)
			}
		}
	}

	// If order still has remaining size, add it to the book
	if order.IsOpen() && order.RemainingSize.GreaterThan(decimal.Zero) {
		order.RemainingSize = order.RemainingSize.Sub(order.Size.Sub(order.RemainingSize))
		if err := s.repo.AddOrder(ctx, order); err != nil {
			return nil, fmt.Errorf("add order to book: %w", err)
		}
	} else if !order.IsOpen() {
		// Order is completely filled, remove it
		if err := s.repo.CancelOrder(ctx, order.ID); err != nil {
			return nil, fmt.Errorf("cancel filled order: %w", err)
		}
	}

	return &CreateOrderResult{
		Order:   order,
		Matches: matches,
		Added:   order.IsOpen(),
	}, nil
}

// CancelOrder cancels an existing order
func (s *OrderBookService) CancelOrder(ctx context.Context, orderID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.repo.CancelOrder(ctx, orderID)
}

// GetOrderBook retrieves the current order book for a market/asset
func (s *OrderBookService) GetOrderBook(ctx context.Context, marketID, assetID string, depth int) (
	*OrderBookSnapshot, error) {
	bids, err := s.repo.GetTopBids(ctx, marketID, assetID, depth)
	if err != nil {
		return nil, fmt.Errorf("get bids: %w", err)
	}

	asks, err := s.repo.GetTopAsks(ctx, marketID, assetID, depth)
	if err != nil {
		return nil, fmt.Errorf("get asks: %w", err)
	}

	return &OrderBookSnapshot{
		MarketID: marketID,
		AssetID:  assetID,
		Bids:     bids,
		Asks:     asks,
	}, nil
}

// GetOrder retrieves an order by ID
func (s *OrderBookService) GetOrder(ctx context.Context, orderID string) (*model.Order, error) {
	return s.repo.GetOrder(ctx, orderID)
}

// GetOrderCount returns the number of orders on each side
func (s *OrderBookService) GetOrderCount(ctx context.Context, marketID, assetID string) (
	bids, asks int64, err error) {
	return s.repo.Count(ctx, marketID, assetID)
}

// validateOrder validates order constraints
func (s *OrderBookService) validateOrder(order *model.Order) error {
	if order.ID == "" {
		return fmt.Errorf("order ID is required")
	}
	if order.Owner == "" {
		return fmt.Errorf("owner is required")
	}
	if order.MarketID == "" {
		return fmt.Errorf("market ID is required")
	}
	if order.AssetID == "" {
		return fmt.Errorf("asset ID is required")
	}
	if order.Price.LessThanOrEqual(decimal.Zero) {
		return fmt.Errorf("price must be greater than zero")
	}
	if order.Size.LessThanOrEqual(decimal.Zero) {
		return fmt.Errorf("size must be greater than zero")
	}
	if order.RemainingSize.LessThanOrEqual(decimal.Zero) {
		return fmt.Errorf("remaining size must be greater than zero")
	}
	if !order.Price.GreaterThan(decimal.Zero) {
		return fmt.Errorf("price must be positive")
	}

	switch order.Type {
	case model.Limit, model.Market:
		// valid
	default:
		return fmt.Errorf("invalid order type: %s", order.Type)
	}

	return nil
}

// matchOrder attempts to match an incoming order against the book
func (s *OrderBookService) matchOrder(ctx context.Context, incomingOrder *model.Order) ([]*repository.MatchResult, error) {
	var matches []*repository.MatchResult

	// Get orders on the opposite side
	oppositeZs, err := s.repo.GetOppositeSide(ctx, incomingOrder.MarketID, incomingOrder.AssetID, incomingOrder.IsBuy())
	if err != nil {
		return nil, err
	}

	// Try to match with each order on the opposite side
	for _, z := range oppositeZs {
		if incomingOrder.RemainingSize.LessThanOrEqual(decimal.Zero) {
			break
		}

		orderID, ok := z.Member.(string)
		if !ok {
			continue
		}

		makerOrder, err := s.repo.GetOrder(ctx, orderID)
		if err != nil {
			continue
		}

		// Check if prices cross
		if !s.canMatch(incomingOrder, makerOrder) {
			continue
		}

		matchSize := min(makerOrder.RemainingSize, incomingOrder.RemainingSize)

		matches = append(matches, &repository.MatchResult{
			TakerOrder:   incomingOrder,
			MakerOrder:   makerOrder,
			MatchedPrice: makerOrder.Price,
			MatchedSize:  matchSize,
		})
	}

	return matches, nil
}

// canMatch checks if two orders can be matched
func (s *OrderBookService) canMatch(taker, maker *model.Order) bool {
	// Must be opposite sides
	if taker.Side == maker.Side {
		return false
	}

	// For limit orders, taker price must cross maker price
	if taker.Type == model.Limit && maker.Type == model.Limit {
		if taker.IsBuy() && maker.IsSell() {
			// Buyer willing to pay at least maker's ask price
			return taker.Price.GreaterThanOrEqual(maker.Price)
		}
		if taker.IsSell() && maker.IsBuy() {
			// Seller willing to accept at most maker's bid price
			return taker.Price.LessThanOrEqual(maker.Price)
		}
	}

	// Market orders always match
	if taker.Type == model.Market {
		return true
	}

	return false
}

// min returns the smaller of two decimals
func min(a, b decimal.Decimal) decimal.Decimal {
	if a.LessThan(b) {
		return a
	}
	return b
}

// CreateOrderResult holds the result of creating an order
type CreateOrderResult struct {
	Order   *model.Order
	Matches []*repository.MatchResult
	Added   bool // true if order was added to the book
}

// OrderBookSnapshot represents a point-in-time view of the order book
type OrderBookSnapshot struct {
	MarketID string
	AssetID  string
	Bids     []*model.Order
	Asks     []*model.Order
}

// BestBid returns the best bid price, or zero if no bids
func (s *OrderBookSnapshot) BestBid() decimal.Decimal {
	if len(s.Bids) == 0 {
		return decimal.Zero
	}
	return s.Bids[0].Price
}

// BestAsk returns the best ask price, or one if no asks
func (s *OrderBookSnapshot) BestAsk() decimal.Decimal {
	if len(s.Asks) == 0 {
		return decimal.NewFromInt(1)
	}
	return s.Asks[0].Price
}

// MidPrice returns the mid price, or zero if no bids or asks
func (s *OrderBookSnapshot) MidPrice() decimal.Decimal {
	bestBid := s.BestBid()
	bestAsk := s.BestAsk()
	if bestBid.IsZero() || bestAsk.IsZero() {
		return decimal.Zero
	}
	return bestBid.Add(bestAsk).Div(decimal.NewFromInt(2))
}

// Spread returns the bid-ask spread
func (s *OrderBookSnapshot) Spread() decimal.Decimal {
	bestBid := s.BestBid()
	bestAsk := s.BestAsk()
	if bestBid.IsZero() || bestAsk.IsZero() {
		return decimal.Zero
	}
	return bestAsk.Sub(bestBid)
}

// BookSize returns the total size on each side
func (s *OrderBookSnapshot) BookSize() (bidSize, askSize decimal.Decimal) {
	for _, o := range s.Bids {
		bidSize = bidSize.Add(o.RemainingSize)
	}
	for _, o := range s.Asks {
		askSize = askSize.Add(o.RemainingSize)
	}
	return
}
