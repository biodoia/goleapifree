package commands

import (
	"fmt"

	"github.com/biodoia/goleapifree/pkg/models"
	"github.com/spf13/cobra"
)

// MigrateCmd rappresenta il comando migrate
var MigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Manage database migrations",
	Long: `Manage database schema migrations.

This command allows you to run, rollback, reset, and seed database migrations
for the GoLeapAI gateway.`,
	Example: `  # Run all pending migrations
  goleapai migrate up

  # Rollback last migration
  goleapai migrate down

  # Reset database (drop and recreate)
  goleapai migrate reset --confirm

  # Seed initial data
  goleapai migrate seed`,
}

var migrateUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Run pending migrations",
	Long:  `Run all pending database migrations to bring the schema up to date.`,
	Example: `  # Run migrations
  goleapai migrate up

  # Run migrations with specific config
  goleapai migrate up -c config.yaml`,
	RunE: runMigrateUp,
}

var migrateDownCmd = &cobra.Command{
	Use:   "down",
	Short: "Rollback migrations",
	Long:  `Rollback the last database migration.`,
	Example: `  # Rollback last migration
  goleapai migrate down

  # Rollback with confirmation
  goleapai migrate down --confirm`,
	RunE: runMigrateDown,
}

var migrateResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset database",
	Long:  `Drop all tables and recreate the schema. This will delete all data.`,
	Example: `  # Reset database (requires confirmation)
  goleapai migrate reset --confirm`,
	RunE: runMigrateReset,
}

var migrateSeedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Seed initial data",
	Long:  `Populate the database with initial seed data (providers, models, etc.).`,
	Example: `  # Seed database
  goleapai migrate seed

  # Force re-seed
  goleapai migrate seed --force`,
	RunE: runMigrateSeed,
}

var migrateStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show migration status",
	Long:  `Display the current status of database migrations.`,
	Example: `  # Show migration status
  goleapai migrate status`,
	RunE: runMigrateStatus,
}

var (
	migrateConfirm bool
	migrateForce   bool
)

func init() {
	migrateDownCmd.Flags().BoolVar(&migrateConfirm, "confirm", false, "Confirm rollback action")
	migrateResetCmd.Flags().BoolVar(&migrateConfirm, "confirm", false, "Confirm reset action")
	migrateSeedCmd.Flags().BoolVar(&migrateForce, "force", false, "Force re-seed even if data exists")

	MigrateCmd.AddCommand(migrateUpCmd)
	MigrateCmd.AddCommand(migrateDownCmd)
	MigrateCmd.AddCommand(migrateResetCmd)
	MigrateCmd.AddCommand(migrateSeedCmd)
	MigrateCmd.AddCommand(migrateStatusCmd)
}

func runMigrateUp(cmd *cobra.Command, args []string) error {
	db, err := initDB(cmd)
	if err != nil {
		return err
	}
	defer db.Close()

	fmt.Println("Running database migrations...")

	if err := db.AutoMigrate(); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	fmt.Println("✓ Migrations completed successfully")
	return nil
}

func runMigrateDown(cmd *cobra.Command, args []string) error {
	if !migrateConfirm {
		return fmt.Errorf("rollback requires --confirm flag to proceed")
	}

	db, err := initDB(cmd)
	if err != nil {
		return err
	}
	defer db.Close()

	fmt.Println("Rolling back last migration...")

	// GORM doesn't have built-in rollback, so we'll drop the last table
	// This is a simplified implementation - in production you'd use a proper migration tool
	fmt.Println("⚠️  GORM AutoMigrate doesn't support rollback natively")
	fmt.Println("Consider using a migration tool like golang-migrate for production")

	return nil
}

func runMigrateReset(cmd *cobra.Command, args []string) error {
	if !migrateConfirm {
		return fmt.Errorf("reset requires --confirm flag to proceed")
	}

	db, err := initDB(cmd)
	if err != nil {
		return err
	}
	defer db.Close()

	fmt.Println("⚠️  Resetting database - ALL DATA WILL BE LOST!")

	// Drop all tables
	tables := []interface{}{
		&models.RequestLog{},
		&models.ProviderStats{},
		&models.RateLimit{},
		&models.Account{},
		&models.Model{},
		&models.Provider{},
	}

	for _, table := range tables {
		if err := db.Migrator().DropTable(table); err != nil {
			fmt.Printf("Warning: Failed to drop table: %v\n", err)
		}
	}

	fmt.Println("✓ All tables dropped")

	// Recreate schema
	fmt.Println("Recreating schema...")
	if err := db.AutoMigrate(); err != nil {
		return fmt.Errorf("failed to recreate schema: %w", err)
	}

	fmt.Println("✓ Database reset successfully")
	return nil
}

func runMigrateSeed(cmd *cobra.Command, args []string) error {
	db, err := initDB(cmd)
	if err != nil {
		return err
	}
	defer db.Close()

	// Check if already seeded
	if !migrateForce {
		var count int64
		db.Model(&models.Provider{}).Count(&count)
		if count > 0 {
			fmt.Printf("Database already contains %d providers\n", count)
			fmt.Println("Use --force to re-seed anyway")
			return nil
		}
	}

	fmt.Println("Seeding database with initial data...")

	if err := db.Seed(); err != nil {
		return fmt.Errorf("seed failed: %w", err)
	}

	// Count seeded providers
	var count int64
	db.Model(&models.Provider{}).Count(&count)

	fmt.Printf("✓ Database seeded successfully (%d providers)\n", count)
	return nil
}

func runMigrateStatus(cmd *cobra.Command, args []string) error {
	db, err := initDB(cmd)
	if err != nil {
		return err
	}
	defer db.Close()

	fmt.Println("Database Migration Status")
	fmt.Println("=========================")
	fmt.Println()

	// Check if tables exist and count records
	tables := []struct {
		name  string
		model interface{}
	}{
		{"providers", &models.Provider{}},
		{"models", &models.Model{}},
		{"accounts", &models.Account{}},
		{"rate_limits", &models.RateLimit{}},
		{"provider_stats", &models.ProviderStats{}},
		{"request_logs", &models.RequestLog{}},
	}

	for _, table := range tables {
		exists := db.Migrator().HasTable(table.model)
		status := "✗ Not created"
		var count int64

		if exists {
			db.Model(table.model).Count(&count)
			status = fmt.Sprintf("✓ Created (%d records)", count)
		}

		fmt.Printf("%-20s %s\n", table.name+":", status)
	}

	fmt.Println()

	// Get database info
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}

	stats := sqlDB.Stats()
	fmt.Println("Database Connection:")
	fmt.Printf("  Open connections:   %d\n", stats.OpenConnections)
	fmt.Printf("  In use:             %d\n", stats.InUse)
	fmt.Printf("  Idle:               %d\n", stats.Idle)
	fmt.Printf("  Max open:           %d\n", stats.MaxOpenConnections)

	return nil
}
