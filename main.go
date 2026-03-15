package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/mishankov/pipefile/application"
	"github.com/mishankov/pipefile/pipefile"
	"github.com/pelletier/go-toml/v2"
)

func main() {
	ctx := context.Background()
	pipefilePath := "pipefile.toml"

	data, err := os.ReadFile(pipefilePath)
	if err != nil {
		panic(err)
	}

	absPath, err := filepath.Abs(pipefilePath)
	if err != nil {
		panic(err)
	}
	baseDir := filepath.Dir(absPath)

	var file pipefile.Pipefile
	err = toml.Unmarshal(data, &file)
	if err != nil {
		panic(err)
	}

	if err := application.New(file, baseDir).Run(ctx); err != nil {
		slog.Error(err.Error())
	}
}
