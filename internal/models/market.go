package models

import "time"

type MarketQuote struct {
	ID        uint      `gorm:"primaryKey"`
	Symbol    string    `gorm:"size:10;index;not null"`
	Price     float64   `gorm:"not null"`
	Timestamp time.Time `gorm:"index;not null"`
}

type MinuteKLine struct {
	ID         uint      `gorm:"primaryKey"`
	Symbol     string    `gorm:"size:10;index:idx_symbol_time;not null"`
	Open       float64   `gorm:"not null"`
	High       float64   `gorm:"not null"`
	Low        float64   `gorm:"not null"`
	Close      float64   `gorm:"not null"`
	MinuteTime time.Time `gorm:"index:idx_symbol_time;not null"`
}
