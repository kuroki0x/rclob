package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kuroki0x/rclob/internal/model"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
)

// OrderBookRepository handles persistence of orders in Redis sorted sets
type OrderBookRepository struct {
	rdb *redis.Client
}

// NewOrderBookRepository creates a new order book repository
func NewOrderBookRepository(rdb *redis.Client) *OrderBookRepository {
	return &OrderBookRepository{rdb: rdb}
}

// KeyBids returns the Redis key for buy orders
func KeyBids(marketID, assetID string) string {
	return fmt.Sprintf("clob:%s:%s:bids", marketID, assetID)
}

// KeyAsks returns the Redis key for sell orders
func KeyAsks(marketID, assetID string) string {
	return fmt.Sprintf("clob:%s:%s:asks", marketID, assetID)
}

// KeyOrderID returns the Redis key for a specific order (hash)
func KeyOrderID(orderID string) string {
	return fmt.Sprintf("clob:order:%s", orderID)
}

// AddOrder adds an order to the order book
func (r *OrderBookRepository) AddOrder(ctx context.Context, order *model.Order) error {
	// Store order details as hash
	orderData, err := json.Marshal(order)
	if err != nil {
		return fmt.Errorf("marshal order: %w", err)
	}

	cmd := r.rdb.TxPipeline()
	cmd.HSet(ctx, KeyOrderID(order.ID), orderData)

	if order.IsBuy() {
		cmd.ZAdd(ctx, KeyBids(order.MarketID, order.AssetID), redis.Z{
			Score:  order.RedisScore(),
			Member: order.ID,
		})
	} else {
		cmd.ZAdd(ctx, KeyAsks(order.MarketID, order.AssetID), redis.Z{
			Score:  order.RedisScore(),
			Member: order.ID,
		})
	}

	_, err = cmd.Exec(ctx)
	return err
}

// UpdateOrder updates an existing order in the order book
func (r *OrderBookRepository) UpdateOrder(ctx context.Context, order *model.Order) error {
	// Remove from old position
	var key string
	if order.IsBuy() {
		key = KeyBids(order.MarketID, order.AssetID)
	} else {
		key = KeyAsks(order.MarketID, order.AssetID)
	}

	// Update order details
	orderData, err := json.Marshal(order)
	if err != nil {
		return fmt.Errorf("marshal order: %w", err)
	}

	pipe := r.rdb.TxPipeline()
	pipe.ZRem(ctx, key, order.ID)
	pipe.HSet(ctx, KeyOrderID(order.ID), orderData)
	pipe.ZAdd(ctx, key, redis.Z{Score: order.RedisScore(), Member: order.ID})

	_, err = pipe.Exec(ctx)
	return err
}

// CancelOrder removes an order from the order book
func (r *OrderBookRepository) CancelOrder(ctx context.Context, orderID string) error {
	// Get order first to know which sorted set it's in
	orderData, err := r.rdb.HGet(ctx, KeyOrderID(orderID), orderID).Result()
	if err != nil {
		return fmt.Errorf("get order: %w", err)
	}

	var order model.Order
	if err := json.Unmarshal([]byte(orderData), &order); err != nil {
		return fmt.Errorf("unmarshal order: %w", err)
	}

	var key string
	if order.IsBuy() {
		key = KeyBids(order.MarketID, order.AssetID)
	} else {
		key = KeyAsks(order.MarketID, order.AssetID)
	}

	pipe := r.rdb.TxPipeline()
	pipe.ZRem(ctx, key, orderID)
	pipe.HDel(ctx, KeyOrderID(orderID), orderID)

	_, err = pipe.Exec(ctx)
	return err
}

// GetOrder retrieves an order by ID
func (r *OrderBookRepository) GetOrder(ctx context.Context, orderID string) (*model.Order, error) {
	orderData, err := r.rdb.HGet(ctx, KeyOrderID(orderID), orderID).Result()
	if err != nil {
		return nil, fmt.Errorf("get order: %w", err)
	}

	var order model.Order
	if err := json.Unmarshal([]byte(orderData), &order); err != nil {
		return nil, fmt.Errorf("unmarshal order: %w", err)
	}

	return &order, nil
}

// GetTopBids retrieves the best (highest price) bids
func (r *OrderBookRepository) GetTopBids(ctx context.Context, marketID, assetID string, count int) ([]*model.Order, error) {
	// ZREVRANGE returns highest scores first (best bids)
	zs, err := r.rdb.ZRevRangeWithScores(ctx, KeyBids(marketID, assetID), 0, int64(count-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("get top bids: %w", err)
	}

	return zsToOrders(ctx, r.rdb, zs)
}

// GetTopAsks retrieves the best (lowest price) asks
func (r *OrderBookRepository) GetTopAsks(ctx context.Context, marketID, assetID string, count int) ([]*model.Order, error) {
	// ZRANGE returns lowest scores first (best asks)
	zs, err := r.rdb.ZRangeWithScores(ctx, KeyAsks(marketID, assetID), 0, int64(count-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("get top asks: %w", err)
	}

	return zsToOrders(ctx, r.rdb, zs)
}

// GetAllOrders retrieves all orders on both sides
func (r *OrderBookRepository) GetAllOrders(ctx context.Context, marketID, assetID string) (
	*bidsAsks, error) {
	bids, err := r.rdb.ZRangeWithScores(ctx, KeyBids(marketID, assetID), 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("get bids: %w", err)
	}

	asks, err := r.rdb.ZRangeWithScores(ctx, KeyAsks(marketID, assetID), 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("get asks: %w", err)
	}

	return &bidsAsks{
		Bids: bids,
		Asks: asks,
	}, nil
}

// MatchOrders retrieves orders on the opposite side for matching
func (r *OrderBookRepository) GetOppositeSide(ctx context.Context, marketID, assetID string, isBuy bool) ([]redis.Z, error) {
	var key string
	if isBuy {
		key = KeyAsks(marketID, assetID)
	} else {
		key = KeyBids(marketID, assetID)
	}

	zs, err := r.rdb.ZRangeWithScores(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("get opposite side: %w", err)
	}

	return zs, nil
}

// RemoveOrderFromSide removes an order from either bids or asks
func (r *OrderBookRepository) RemoveOrderFromSide(ctx context.Context, order *model.Order) error {
	var key string
	if order.IsBuy() {
		key = KeyBids(order.MarketID, order.AssetID)
	} else {
		key = KeyAsks(order.MarketID, order.AssetID)
	}

	return r.rdb.ZRem(ctx, key, order.ID).Err()
}

// Count returns the number of orders on each side
func (r *OrderBookRepository) Count(ctx context.Context, marketID, assetID string) (bids, asks int64, err error) {
	bids, err = r.rdb.ZCard(ctx, KeyBids(marketID, assetID)).Result()
	if err != nil {
		return 0, 0, err
	}

	asks, err = r.rdb.ZCard(ctx, KeyAsks(marketID, assetID)).Result()
	return bids, asks, nil
}

// MatchResult represents a match between two orders
type MatchResult struct {
	TakerOrder   *model.Order
	MakerOrder   *model.Order
	MatchedPrice decimal.Decimal
	MatchedSize  decimal.Decimal
}

// bidsAsks holds aggregated bid and ask orders
type bidsAsks struct {
	Bids []redis.Z
	Asks []redis.Z
}

// zsToOrders converts Redis Z slices to Order slices
func zsToOrders(ctx context.Context, rdb *redis.Client, zs []redis.Z) ([]*model.Order, error) {
	orders := make([]*model.Order, len(zs))
	for i, z := range zs {
		orderID, ok := z.Member.(string)
		if !ok {
			return nil, fmt.Errorf("invalid order ID type: %T", z.Member)
		}

		orderData, err := rdb.HGet(ctx, KeyOrderID(orderID), orderID).Result()
		if err != nil {
			return nil, fmt.Errorf("get order data: %w", err)
		}

		var order model.Order
		if err := json.Unmarshal([]byte(orderData), &order); err != nil {
			return nil, fmt.Errorf("unmarshal order: %w", err)
		}

		orders[i] = &order
	}
	return orders, nil
}
