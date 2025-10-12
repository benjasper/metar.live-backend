package main

import (
	"context"
	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/sync/errgroup"
	"metar.live/ent"
	"metar.live/ent/migrate"
	"metar.live/environment"
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
	// Run the auto migration tool.
	if err = client.Schema.Create(context.Background(), migrate.WithDropIndex(true),
		migrate.WithDropColumn(true),
	); err != nil {
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
