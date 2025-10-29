package graph

import (
	"time"

	"github.com/99designs/gqlgen/graphql"
	"metar.live/ent"
	"metar.live/graph/generated"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type StatusProvider interface {
	LastWeatherSync() time.Time
}

type Resolver struct {
	client         *ent.Client
	statusProvider StatusProvider
}

func NewSchema(client *ent.Client, statusProvider StatusProvider) graphql.ExecutableSchema {
	return generated.NewExecutableSchema(generated.Config{
		Resolvers: &Resolver{client: client, statusProvider: statusProvider},
	})
}
