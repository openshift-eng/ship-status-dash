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

	// If triage_notes or outage_links tables exist but are missing required columns (e.g. from
	// an aborted migration), drop and recreate them. They hold no production-critical data.
	if db.Migrator().HasTable("triage_notes") && !db.Migrator().HasColumn(&types.TriageNote{}, "body") {
		log.Info("triage_notes table is incomplete; dropping for recreation")
		if err = db.Exec("DROP TABLE triage_notes").Error; err != nil {
			log.WithField("error", err).Fatal("Failed to drop incomplete triage_notes table")
		}
	}
	if db.Migrator().HasTable("outage_links") {
		// Drop the table if it is missing required columns or has stale unexpected columns
		// (e.g. a "name" column from a previous schema iteration).
		var staleColCount int64
		if err = db.Raw("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = CURRENT_SCHEMA() AND table_name = 'outage_links' AND column_name NOT IN ('id','created_at','updated_at','deleted_at','outage_id','url','link_type','description','added_by')").Scan(&staleColCount).Error; err != nil {
			log.WithField("error", err).Fatal("Failed to inspect outage_links columns")
		}
		if !db.Migrator().HasColumn(&types.OutageLink{}, "url") || staleColCount > 0 {
			log.Info("outage_links table has incorrect schema; dropping for recreation")
			if err = db.Exec("DROP TABLE outage_links").Error; err != nil {
				log.WithField("error", err).Fatal("Failed to drop outage_links table")
			}
		}
	}

	if err = db.AutoMigrate(&types.TriageNote{}); err != nil {
		log.WithField("error", err).Fatal("Failed to migrate TriageNote table")
	}

	if err = db.AutoMigrate(&types.OutageLink{}); err != nil {
		log.WithField("error", err).Fatal("Failed to migrate OutageLink table")
	}

	// Drop the old scalar triage_notes column from outages (replaced by the triage_notes table).
	if db.Migrator().HasColumn(&types.Outage{}, "triage_notes") {
		if err = db.Migrator().DropColumn(&types.Outage{}, "triage_notes"); err != nil {
			log.WithField("error", err).Fatal("Failed to drop triage_notes column from outages table")
		}
		log.Info("Dropped legacy triage_notes column from outages table")
	}

	// Drop the added_by column from outage_links (now tracked via audit logs).
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

	// Rename old link_type value from "incident_channel" to "incident_channel_thread".
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
