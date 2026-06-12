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

	if err = db.AutoMigrate(&types.OutageAuditLog{}); err != nil {
		log.WithField("error", err).Fatal("Failed to migrate OutageAuditLog table")
	}

	if err = db.AutoMigrate(&types.TriageNote{}); err != nil {
		log.WithField("error", err).Fatal("Failed to migrate TriageNote table")
	}

	if err = db.AutoMigrate(&types.OutageLink{}); err != nil {
		log.WithField("error", err).Fatal("Failed to migrate OutageLink table")
	}

	if err = db.AutoMigrate(&types.OutageReport{}); err != nil {
		log.WithField("error", err).Fatal("Failed to migrate OutageReport table")
	}

	if err = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_one_active_suspected_per_subcomponent
		ON outages (component_name, sub_component_name)
		WHERE end_time IS NULL AND severity = 'Suspected' AND confirmed_at IS NULL`).Error; err != nil {
		log.WithField("error", err).Fatal("Failed to create unique partial index for suspected outages")
	}

	// TODO: remove once all environments have run this migration (triage_notes moved to own table)
	if db.Migrator().HasColumn(&types.Outage{}, "triage_notes") {
		if err = db.Migrator().DropColumn(&types.Outage{}, "triage_notes"); err != nil {
			log.WithField("error", err).Fatal("Failed to drop triage_notes column from outages table")
		}
		log.Info("Dropped legacy triage_notes column from outages table")
	}

	// TODO: remove once all environments have run this migration (added_by replaced by audit logs)
	if db.Migrator().HasTable("outage_links") {
		var addedByExists int64
		db.Raw("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = CURRENT_SCHEMA() AND table_name = 'outage_links' AND column_name = 'added_by'").Scan(&addedByExists)
		if addedByExists > 0 {
			if err = db.Exec("ALTER TABLE outage_links DROP COLUMN added_by").Error; err != nil {
				log.WithField("error", err).Fatal("Failed to drop added_by column from outage_links")
			}
			log.Info("Dropped added_by column from outage_links table")
		}
	}

	// TODO: remove once all environments have run this migration (incident_channel renamed to incident_channel_thread)
	if db.Migrator().HasTable("outage_links") {
		if err = db.Exec("UPDATE outage_links SET link_type = 'incident_channel_thread' WHERE link_type = 'incident_channel'").Error; err != nil {
			log.WithField("error", err).Warn("Failed to rename incident_channel link type values")
		}
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
