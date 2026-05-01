package database

import (
	"log"

	"github.com/xboard/xboard/internal/config"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlog "gorm.io/gorm/logger"
)

var DB *gorm.DB

func InitDB(cfg *config.DatabaseConfig) *gorm.DB {
	var dial gorm.Dialector
	switch cfg.Driver {
	case "mysql":
		dial = mysql.New(mysql.Config{
			DSN:                       cfg.DSN(),
			DefaultStringSize:         255,
			SkipInitializeWithVersion: false,
		})
	case "sqlite":
		dial = sqlite.Open(cfg.SQLitePath)
	default:
		log.Fatalf("unsupported database driver: %s", cfg.Driver)
	}

	var logLevel gormlog.LogLevel
	if cfg.Driver == "mysql" && cfg.ConnMaxLifetime > 0 {
		// use debug level during migration
		logLevel = gormlog.Info
	} else {
		logLevel = gormlog.Warn
	}

	db, err := gorm.Open(dial, &gorm.Config{
		Logger:                 gormlog.Default.LogMode(logLevel),
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
	})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("failed to get underlying sql.DB: %v", err)
	}

	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}

	DB = db
	return db
}
