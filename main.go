package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/mishankov/pipefile/application"
	"github.com/mishankov/pipefile/pipefile"
	"github.com/pelletier/go-toml/v2"
)

func main() {
	ctx := context.Background()

	data, err := os.ReadFile("pipefile.toml")
	if err != nil {
		panic(err)
	}
	var file pipefile.Pipefile
	err = toml.Unmarshal(data, &file)
	if err != nil {
		panic(err)
	}

	if err := application.New(file).Run(ctx); err != nil {
		slog.Error(err.Error())
	}
}
