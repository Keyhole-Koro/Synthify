package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	ctx := context.Background()
	db, err := pgxpool.New(ctx, "postgres://synthify:synthify@127.0.0.1:5432/synthify?sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	rows, err := db.Query(ctx, "SELECT workspace_id, account_id, name FROM workspaces WHERE name = 'Synthify Dev Seed' ORDER BY created_at DESC")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var id, accountID, name string
		if err := rows.Scan(&id, &accountID, &name); err != nil {
			log.Fatal(err)
		}
		
		var count int
		db.QueryRow(ctx, "SELECT COUNT(*) FROM tree_items WHERE workspace_id = $1 AND title = 'Frontend UI'", id).Scan(&count)
		
		fmt.Printf("Workspace: %s (ID: %s, Account: %s) - Frontend UI count: %d\n", name, id, accountID, count)
	}
}
