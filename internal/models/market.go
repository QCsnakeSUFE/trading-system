package models

import "time"

type MarketQuote struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Symbol    string    `gorm:"size:10;index;not null" json:"symbol"`
	Price     float64   `gorm:"not null" json:"price"`
	Timestamp time.Time `gorm:"index;not null" json:"timestamp"`
}

type MinuteKLine struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Symbol     string    `gorm:"size:10;index:idx_symbol_time;not null" json:"symbol"`
	Open       float64   `gorm:"not null" json:"open"`
	High       float64   `gorm:"not null" json:"high"`
	Low        float64   `gorm:"not null" json:"low"`
	Close      float64   `gorm:"not null" json:"close"`
	Volume     int64     `gorm:"default:0" json:"volume"`
	MinuteTime time.Time `gorm:"index:idx_symbol_time;not null" json:"minute_time"`
}
