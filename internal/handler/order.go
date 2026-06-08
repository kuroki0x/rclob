package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/kuroki0x/rclob/internal/model"
	"github.com/kuroki0x/rclob/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// OrderHandler handles HTTP requests for order book operations
type OrderHandler struct {
	svc *service.OrderBookService
	log *zap.Logger
}

// NewOrderHandler creates a new order handler
func NewOrderHandler(svc *service.OrderBookService, log *zap.Logger) *OrderHandler {
	return &OrderHandler{
		svc: svc,
		log: log,
	}
}

// RegisterRoutes registers the order routes
func (h *OrderHandler) RegisterRoutes(r chi.Router) {
	r.Post("/orders", h.CreateOrder)
	r.Get("/markets/{marketID}/assets/{assetID}/book", h.GetOrderBook)
	r.Get("/orders/{orderID}", h.GetOrder)
	r.Delete("/orders/{orderID}", h.CancelOrder)
	r.Get("/markets/{marketID}/assets/{assetID}/stats", h.GetStats)
}

// createOrderRequest represents the request body for creating an order
type createOrderRequest struct {
	Owner    string `json:"owner"`
	MarketID string `json:"market_id"`
	AssetID  string `json:"asset_id"`
	Side     string `json:"side"`
	Price    string `json:"price"`
	Size     string `json:"size"`
	Type     string `json:"type"`
}

// CreateOrder handles POST /orders
func (h *OrderHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var req createOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body", err.Error())
		return
	}

	// Parse fields
	side, err := parseSide(req.Side)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid side", err.Error())
		return
	}

	price, err := decimal.NewFromString(req.Price)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid price", err.Error())
		return
	}

	size, err := decimal.NewFromString(req.Size)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid size", err.Error())
		return
	}

	orderType := model.OrderType(req.Type)
	if orderType == "" {
		orderType = model.Limit
	}

	// Create the order
	order := model.NewOrder(
		uuid.New().String(),
		req.Owner,
		req.MarketID,
		req.AssetID,
		side,
		price,
		size,
		orderType,
	)

	result, err := h.svc.CreateOrder(r.Context(), order)
	if err != nil {
		h.log.Error("failed to create order",
			zap.String("order_id", order.ID),
			zap.Error(err),
		)
		h.writeError(w, http.StatusInternalServerError, "failed to create order", err.Error())
		return
	}

	h.log.Info("order processed",
		zap.String("order_id", order.ID),
		zap.String("side", side.String()),
		zap.String("price", price.String()),
		zap.String("size", size.String()),
		zap.Bool("added_to_book", result.Added),
		zap.Int("matches", len(result.Matches)),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(result)
}

// GetOrderBook handles GET /markets/{marketID}/assets/{assetID}/book
func (h *OrderHandler) GetOrderBook(w http.ResponseWriter, r *http.Request) {
	marketID := chi.URLParam(r, "marketID")
	assetID := chi.URLParam(r, "assetID")

	depth := 10
	if d := r.URL.Query().Get("depth"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 {
			depth = parsed
		}
	}

	snapshot, err := h.svc.GetOrderBook(r.Context(), marketID, assetID, depth)
	if err != nil {
		h.log.Error("failed to get order book",
			zap.String("market_id", marketID),
			zap.String("asset_id", assetID),
			zap.Error(err),
		)
		h.writeError(w, http.StatusInternalServerError, "failed to get order book", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(snapshot)
}

// GetOrder handles GET /orders/{orderID}
func (h *OrderHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderID")

	order, err := h.svc.GetOrder(r.Context(), orderID)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "order not found", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)
}

// CancelOrder handles DELETE /orders/{orderID}
func (h *OrderHandler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderID")

	if err := h.svc.CancelOrder(r.Context(), orderID); err != nil {
		h.log.Error("failed to cancel order",
			zap.String("order_id", orderID),
			zap.Error(err),
		)
		h.writeError(w, http.StatusInternalServerError, "failed to cancel order", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetStats handles GET /markets/{marketID}/assets/{assetID}/stats
func (h *OrderHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	marketID := chi.URLParam(r, "marketID")
	assetID := chi.URLParam(r, "assetID")

	bids, asks, err := h.svc.GetOrderCount(r.Context(), marketID, assetID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to get stats", err.Error())
		return
	}

	stats := map[string]interface{}{
		"market_id": marketID,
		"asset_id":  assetID,
		"bid_count": bids,
		"ask_count": asks,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// parseSide converts a string to model.Side
func parseSide(s string) (model.Side, error) {
	switch s {
	case "BUY", "buy", "B":
		return model.Buy, nil
	case "SELL", "sell", "S":
		return model.Sell, nil
	default:
		return 0, strconv.ErrSyntax
	}
}

// writeError writes a JSON error response
func (h *OrderHandler) writeError(w http.ResponseWriter, status int, title string, detail string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error":  title,
		"detail": detail,
	})
}
