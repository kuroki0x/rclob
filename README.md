# rclob

A simplified CLOB (Central Limit Order Book) service backed by Redis sorted sets.

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐     ┌──────────┐
│   HTTP API   │────>│   Handler    │────>│   Service   │────>│ Repository │
│   (chi)      │<----│   (REST)     │<----│ (Matching)  │<----│ (Redis)   │
└─────────────┘     └──────────────┘     └─────────────┘     └──────────┘
```

## Tech Stack

- **Go 1.23+**
- **Redis** (sorted sets for price-time priority ordering)
- **chi** (HTTP router)
- **uber/zap** (structured logging)
- **caarlos0/env** (configuration)
- **shopspring/decimal** (precise decimal arithmetic)

## Quick Start

```bash
# Start Redis
docker run -d --name redis -p 6379:6379 redis:7

# Configure
export REDIS_ADDR="localhost:6379"
export PORT="8080"
export LOG_LEVEL="info"

# Run
go run ./cmd/server/main.go
```

## API

### Create Order

```bash
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{
    "owner": "0x1234...",
    "market_id": "0xabc...",
    "asset_id": "0xdef...",
    "side": "BUY",
    "price": "0.50",
    "size": "100",
    "type": "LIMIT"
  }'
```

### Get Order Book

```bash
curl http://localhost:8080/markets/{market_id}/assets/{asset_id}/book?depth=10
```

### Get Order

```bash
curl http://localhost:8080/orders/{order_id}
```

### Cancel Order

```bash
curl -X DELETE http://localhost:8080/orders/{order_id}
```

### Get Stats

```bash
curl http://localhost:8080/markets/{market_id}/assets/{asset_id}/stats
```

### Health Check

```bash
curl http://localhost:8080/health
```

## Redis Keys

| Key Pattern | Type | Description |
|---|---|---|
| `clob:{market}:{asset}:bids` | Sorted Set | Buy orders (highest score = best price) |
| `clob:{market}:{asset}:asks` | Sorted Set | Sell orders (lowest score = best price) |
| `clob:order:{id}` | Hash | Order details |

## Order Score Calculation

Orders are stored in Redis sorted sets with composite scores:

- **Buy orders**: `price * 1e18 + (MAX_INT - timestamp)` — higher price first, then earlier timestamp
- **Sell orders**: `price * 1e18 + timestamp` — lower price first, then earlier timestamp

This ensures proper price-time priority ordering within each side.
