package db

import (
	"fmt"
	"log"
	"os"
	"time"
	"trading_system/internal/models"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func InitDB() *gorm.DB {
	dsn := os.Getenv("DB_DSN")

	if dsn == "" {
		dsn = "trading:trading123@tcp(127.0.0.1:3306)/trading_db?charset=utf8mb4&parseTime=True&loc=Local&timeout=5s"
	}

	fmt.Printf("正在尝试连接数据库: %s\n", dsn)

	var db *gorm.DB
	var err error

	for i := 0; i < 20; i++ {
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err == nil {
			break
		}
		log.Printf("数据库还没准备好 (%d/20)，500ms 后重试: %v\n", i+1, err)
		time.Sleep(500 * time.Millisecond)
	}

	if err != nil {
		log.Fatalf("无法连接到数据库：%v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Printf("获取数据库连接失败：%v", err)
	} else {
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetMaxOpenConns(100)
		sqlDB.SetConnMaxLifetime(time.Hour)
	}

	err = db.AutoMigrate(&models.MarketQuote{}, &models.MinuteKLine{})
	if err != nil {
		log.Printf("自动迁移失败：%v", err)
	}
	fmt.Println("数据库迁移成功，行情与 K 线表已就位")

	return db
}
