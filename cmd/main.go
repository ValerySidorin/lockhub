package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/ValerySidorin/lockhub/server"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	err := server.ListenAndServe(ctx, ":13796", nil, nil)
	if err != nil {
		log.Fatal(err)
	}
}
