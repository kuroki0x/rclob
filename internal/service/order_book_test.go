package service

import (
	"testing"

	"github.com/kuroki0x/rclob/internal/model"
	"github.com/shopspring/decimal"
)

func TestCanMatch(t *testing.T) {
	s := NewOrderBookService(nil)

	tests := []struct {
		name  string
		taker *model.Order
		maker *model.Order
		want  bool
	}{
		{
			name: "buy taker matches sell maker at same price",
			taker: model.NewOrder("t1", "o1", "m1", "a1", model.Buy,
				decimal.NewFromInt(50), decimal.NewFromInt(100), model.Limit),
			maker: model.NewOrder("m1", "o2", "m1", "a1", model.Sell,
				decimal.NewFromInt(50), decimal.NewFromInt(100), model.Limit),
			want: true,
		},
		{
			name: "buy taker matches sell maker at higher price",
			taker: model.NewOrder("t1", "o1", "m1", "a1", model.Buy,
				decimal.NewFromInt(60), decimal.NewFromInt(100), model.Limit),
			maker: model.NewOrder("m1", "o2", "m1", "a1", model.Sell,
				decimal.NewFromInt(50), decimal.NewFromInt(100), model.Limit),
			want: true,
		},
		{
			name: "buy taker does not match sell maker at lower price",
			taker: model.NewOrder("t1", "o1", "m1", "a1", model.Buy,
				decimal.NewFromInt(40), decimal.NewFromInt(100), model.Limit),
			maker: model.NewOrder("m1", "o2", "m1", "a1", model.Sell,
				decimal.NewFromInt(50), decimal.NewFromInt(100), model.Limit),
			want: false,
		},
		{
			name: "sell taker matches buy maker at same price",
			taker: model.NewOrder("t1", "o1", "m1", "a1", model.Sell,
				decimal.NewFromInt(50), decimal.NewFromInt(100), model.Limit),
			maker: model.NewOrder("m1", "o2", "m1", "a1", model.Buy,
				decimal.NewFromInt(50), decimal.NewFromInt(100), model.Limit),
			want: true,
		},
		{
			name: "sell taker does not match buy maker at higher price",
			taker: model.NewOrder("t1", "o1", "m1", "a1", model.Sell,
				decimal.NewFromInt(60), decimal.NewFromInt(100), model.Limit),
			maker: model.NewOrder("m1", "o2", "m1", "a1", model.Buy,
				decimal.NewFromInt(50), decimal.NewFromInt(100), model.Limit),
			want: false,
		},
		{
			name: "same side cannot match",
			taker: model.NewOrder("t1", "o1", "m1", "a1", model.Buy,
				decimal.NewFromInt(50), decimal.NewFromInt(100), model.Limit),
			maker: model.NewOrder("m1", "o2", "m1", "a1", model.Buy,
				decimal.NewFromInt(50), decimal.NewFromInt(100), model.Limit),
			want: false,
		},
		{
			name: "market order always matches",
			taker: model.NewOrder("t1", "o1", "m1", "a1", model.Buy,
				decimal.NewFromInt(50), decimal.NewFromInt(100), model.Market),
			maker: model.NewOrder("m1", "o2", "m1", "a1", model.Sell,
				decimal.NewFromInt(50), decimal.NewFromInt(100), model.Limit),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.canMatch(tt.taker, tt.maker)
			if got != tt.want {
				t.Errorf("canMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateOrder(t *testing.T) {
	s := NewOrderBookService(nil)

	tests := []struct {
		name    string
		order   *model.Order
		wantErr bool
	}{
		{
			name: "valid order",
			order: model.NewOrder("o1", "owner", "m1", "a1", model.Buy,
				decimal.NewFromInt(50), decimal.NewFromInt(100), model.Limit),
			wantErr: false,
		},
		{
			name: "zero price",
			order: model.NewOrder("o1", "owner", "m1", "a1", model.Buy,
				decimal.Zero, decimal.NewFromInt(100), model.Limit),
			wantErr: true,
		},
		{
			name: "zero size",
			order: model.NewOrder("o1", "owner", "m1", "a1", model.Buy,
				decimal.NewFromInt(50), decimal.Zero, model.Limit),
			wantErr: true,
		},
		{
			name: "empty owner",
			order: model.NewOrder("o1", "", "m1", "a1", model.Buy,
				decimal.NewFromInt(50), decimal.NewFromInt(100), model.Limit),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.validateOrder(tt.order)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateOrder() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMin(t *testing.T) {
	tests := []struct {
		a, b, want decimal.Decimal
	}{
		{decimal.NewFromInt(10), decimal.NewFromInt(20), decimal.NewFromInt(10)},
		{decimal.NewFromInt(20), decimal.NewFromInt(10), decimal.NewFromInt(10)},
		{decimal.NewFromInt(10), decimal.NewFromInt(10), decimal.NewFromInt(10)},
	}

	for _, tt := range tests {
		got := min(tt.a, tt.b)
		if !got.Equal(tt.want) {
			t.Errorf("min(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}
