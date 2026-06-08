package model

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestNewOrder(t *testing.T) {
	order := NewOrder(
		"ord-1",
		"owner-1",
		"market-1",
		"asset-1",
		Buy,
		decimal.NewFromInt(50),
		decimal.NewFromInt(100),
		Limit,
	)

	if order.ID != "ord-1" {
		t.Errorf("expected ID ord-1, got %s", order.ID)
	}
	if order.Owner != "owner-1" {
		t.Errorf("expected owner owner-1, got %s", order.Owner)
	}
	if order.Price.String() != "50" {
		t.Errorf("expected price 50, got %s", order.Price.String())
	}
	if order.Size.String() != "100" {
		t.Errorf("expected size 100, got %s", order.Size.String())
	}
	if order.Status != StatusOpen {
		t.Errorf("expected status OPEN, got %s", order.Status)
	}
	if !order.IsOpen() {
		t.Error("expected order to be open")
	}
}

func TestFill(t *testing.T) {
	order := NewOrder(
		"ord-1",
		"owner-1",
		"market-1",
		"asset-1",
		Buy,
		decimal.NewFromInt(50),
		decimal.NewFromInt(100),
		Limit,
	)

	// Fill 60
	order.Fill(decimal.NewFromInt(60))
	if order.RemainingSize.String() != "40" {
		t.Errorf("expected remaining 40, got %s", order.RemainingSize.String())
	}
	if order.Status != StatusOpen {
		t.Errorf("expected status OPEN after partial fill, got %s", order.Status)
	}

	// Fill remaining 40
	order.Fill(decimal.NewFromInt(40))
	if !order.RemainingSize.IsZero() {
		t.Errorf("expected remaining to be zero after full fill, got %v", order.RemainingSize)
	}
	if order.Status != StatusFilled {
		t.Errorf("expected status FILLED after full fill, got %s", order.Status)
	}
}

func TestRedisScore(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	// Buy order: higher price = higher score
	buy1 := &Order{
		Side:      Buy,
		Price:     decimal.NewFromInt(50),
		CreatedAt: now,
	}
	buy2 := &Order{
		Side:      Buy,
		Price:     decimal.NewFromInt(60),
		CreatedAt: now,
	}
	if buy1.RedisScore() >= buy2.RedisScore() {
		t.Error("buy order with higher price should have higher score")
	}

	// Sell order: lower price = lower score
	sell1 := &Order{
		Side:      Sell,
		Price:     decimal.NewFromInt(50),
		CreatedAt: now,
	}
	sell2 := &Order{
		Side:      Sell,
		Price:     decimal.NewFromInt(60),
		CreatedAt: now,
	}
	if sell1.RedisScore() >= sell2.RedisScore() {
		t.Error("sell order with lower price should have lower score")
	}
}

func TestSide(t *testing.T) {
	if Buy.String() != "BUY" {
		t.Errorf("expected BUY, got %s", Buy.String())
	}
	if Sell.String() != "SELL" {
		t.Errorf("expected SELL, got %s", Sell.String())
	}
}

func TestFillExceedsRemaining(t *testing.T) {
	order := NewOrder(
		"ord-1",
		"owner-1",
		"market-1",
		"asset-1",
		Buy,
		decimal.NewFromInt(50),
		decimal.NewFromInt(100),
		Limit,
	)

	// Try to fill more than remaining
	order.Fill(decimal.NewFromInt(200))
	if !order.RemainingSize.IsZero() {
		t.Errorf("expected remaining to be zero after exceeding fill, got %v", order.RemainingSize)
	}
	if order.Status != StatusFilled {
		t.Errorf("expected status FILLED, got %s", order.Status)
	}
}
