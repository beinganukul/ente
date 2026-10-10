package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ente/museum/pkg/utils/config"
	"github.com/ente/museum/pkg/utils/database"
)

func main() {
	applyMigrations := flag.Bool("migrate", false, "apply the selected provider's migrations; run only one migrator")
	exportFile := flag.String("export", "", "export stopped Museum database to a new private snapshot file")
	importFile := flag.String("import", "", "import private snapshot into an empty migrated database; stop Museum first")
	flag.Parse()
	if err := runTransfer(*applyMigrations, *exportFile, *importFile); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runTransfer(applyMigrations bool, exportFile, importFile string) error {
	if (exportFile != "" && importFile != "") || (applyMigrations && (exportFile != "" || importFile != "")) {
		return fmt.Errorf("choose exactly one of -migrate, -export or -import")
	}
	environment := os.Getenv("ENVIRONMENT")
	if environment == "" {
		environment = "local"
	}
	if err := config.ConfigureViper(environment); err != nil {
		return err
	}
	cfg := database.ConfigFromViper()
	timeout := 10 * time.Second
	if exportFile != "" || importFile != "" {
		timeout = 24 * time.Hour
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	provider, err := cfg.EffectiveProvider()
	if err != nil {
		return err
	}
	if exportFile != "" {
		if err := database.ExportSnapshot(ctx, cfg, exportFile); err != nil {
			return err
		}
		fmt.Println("Private database snapshot exported; source retained")
	} else if importFile != "" {
		if err := database.ImportSnapshot(ctx, cfg, importFile); err != nil {
			return err
		}
		fmt.Println("Database snapshot imported and verified")
	} else if applyMigrations {
		if err := database.Migrate(ctx, cfg, "migrations"); err != nil {
			return err
		}
		fmt.Printf("%s migrations applied\n", provider)
	} else {
		db, err := database.Open(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()
		fmt.Printf("%s connection verified\n", provider)
	}
	return nil
}
