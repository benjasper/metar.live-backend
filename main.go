package main

import (
	"context"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/field"
	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/sync/errgroup"
	"metar.live/ent"
	"metar.live/ent/migrate"
	"metar.live/environment"
	"metar.live/importer"
	"metar.live/logging"
	"metar.live/server"
)

func main() {
	environment.Initialize()

	logger := logging.NewLogger()

	logger.Info("Starting up...")

	logger.Info("Connecting to database...")

	client, err := ent.Open("mysql", environment.Global.Database)
	if err != nil {
		logger.Fatal(err)
	}
	defer func(client *ent.Client) {
		_ = client.Close()
	}(client)

	logger.Info("Connected to database")

	logger.Info("Running migrations...")
	keepUUIDColumns()
	// Run the auto migration tool.
	if err = client.Schema.Create(context.Background(), migrate.WithDropIndex(true),
		migrate.WithDropColumn(true),
	); err != nil {
		logger.Fatal(err)
	}

	if err = importer.MigrateWeatherHashes(context.Background(), client); err != nil {
		logger.Fatal(err)
	}

	logger.Info("Migrations ran successfully")

	wg := errgroup.Group{}
	wg.Go(func() error {
		return RunServer(client, logger)
	})

	err = wg.Wait()
	if err != nil {
		logger.Fatal(err)
	}
}

func RunServer(db *ent.Client, logger *logging.Logger) error {
	err := server.NewServer(db, logger).Run()
	if err != nil {
		return err
	}

	return nil
}

// keepUUIDColumns keeps UUID columns as char(36), because newer ent versions would migrate them to MariaDB's native
// uuid type, which fails on columns that are referenced by foreign keys.
func keepUUIDColumns() {
	for _, table := range migrate.Tables {
		for _, column := range table.Columns {
			if column.Type == field.TypeUUID {
				column.SchemaType = map[string]string{dialect.MySQL: "char(36)"}
				column.Collation = "utf8mb4_bin"
			}
		}
	}
}
