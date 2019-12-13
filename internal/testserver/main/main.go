package main

import (
	"context"
	"log"
	"net/http"
	"os"

	gqlgen "github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/handler"
	"github.com/shurcooL/graphql/internal/testserver"
)

const (
	defaultHost = "127.0.0.1"
	defaultPort = "8080"
)

type testTracer struct {
	gqlgen.NopTracer
}

func (testTracer) StartOperationParsing(ctx context.Context) context.Context {
	log.Println("hello")
	return ctx
}

func (testTracer) StartOperationValidation(ctx context.Context) context.Context {
	log.Println("oh hi")
	return ctx
}

func main() {
	host := os.Getenv("HOST")
	if host == "" {
		host = defaultHost
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	http.Handle("/", handler.Playground("GraphQL playground", "/query"))
	http.Handle("/query", handler.GraphQL(
		testserver.NewExecutableSchema(testserver.Config{Resolvers: &testserver.Resolver{}}),
		handler.RequestMiddleware(func(ctx context.Context, next func(ctx context.Context) []byte) []byte {
			log.Println("serving request")
			result := next(ctx)
			log.Println(string(result))
			return result
		}),
		handler.Tracer(testTracer{}),
	))

	log.Printf("connect to http://%s:%s/ for GraphQL playground", host, port)
	log.Fatal(http.ListenAndServe(host+":"+port, nil))
}
