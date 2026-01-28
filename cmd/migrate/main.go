package main

import (
	"flag"
	"fmt"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"ship-status-dash/pkg/types"
)

func main() {
	log := logrus.New()
	log.SetLevel(logrus.InfoLevel)
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	dsn := flag.String("dsn", "postgres://postgres:postgres@localhost:5432/ship_status?sslmode=disable&client_encoding=UTF8", "PostgreSQL DSN connection string")
	flag.Parse()

	if *dsn == "" {
		log.Fatal("DSN cannot be empty")
	}

	log.Info("Connecting to PostgreSQL database")

	db, err := gorm.Open(postgres.Open(*dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.WithField("error", err).Fatal("Failed to connect to database")
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.WithField("error", err).Fatal("Failed to get database instance")
	}

	// Explicitly set client encoding (required for simple protocol queries)
	if _, err := sqlDB.Exec("SET client_encoding = 'UTF8'"); err != nil {
		log.WithField("error", err).Fatal("Failed to set client encoding")
	}

	log.Info("Running migrations...")

	if err = db.AutoMigrate(&types.Outage{}); err != nil {
		log.WithField("error", err).Fatal("Failed to migrate Outage table")
	}

	if err = db.AutoMigrate(&types.Reason{}); err != nil {
		log.WithField("error", err).Fatal("Failed to migrate Reason table")
	}

	if err = db.AutoMigrate(&types.ComponentReportPing{}); err != nil {
		log.WithField("error", err).Fatal("Failed to migrate ComponentReportPing table")
	}

	if err = db.AutoMigrate(&types.SlackThread{}); err != nil {
		log.WithField("error", err).Fatal("Failed to migrate SlackThread table")
	}

	log.Info("Migration completed successfully")

	var tableCount int64
	db.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE'").Scan(&tableCount)

	log.Infof("Database contains %d tables", tableCount)

	if err := sqlDB.Close(); err != nil {
		log.WithField("error", err).Warn("Failed to close database")
	}

	fmt.Println("\n✓ Migration complete")
}
