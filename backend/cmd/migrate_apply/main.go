package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL not set")
	}

	db, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	apply := flag.Bool("apply", false, "apply migration")
	flag.Parse()

	migDir := filepath.Join("internal", "migrations")
	files, err := os.ReadDir(migDir)
	if err != nil {
		log.Fatalf("read migrations dir: %v", err)
	}
	for _, f := range files {
		name := f.Name()
		if !*apply {
			fmt.Println(name)
			continue
		}
		b, err := os.ReadFile(filepath.Join(migDir, name))
		if err != nil {
			log.Fatalf("read file %s: %v", name, err)
		}
		if _, err := db.Exec(context.Background(), string(b)); err != nil {
			log.Fatalf("failed to apply %s: %v", name, err)
		}
		fmt.Printf("applied %s\n", name)
	}
}
