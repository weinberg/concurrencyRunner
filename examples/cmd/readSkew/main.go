package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/jackc/pgx/v4"
	"os"
)

// alice performs a read which is susceptible to read skew when operating in REPEATABLE COMMITTED isolation level.
// By changing to REPEATABLE READ the read skew is avoided.
//
func alice(conn *pgx.Conn) error {
	ctx := context.Background()

	// create transaction
	/* using repeatable read will fix this bug:
	fmt.Printf("BEGIN ISOLATION LEVEL REPEATABLE READ\n")
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.RepeatableRead,
	})
	if err != nil {
		tx.Rollback(ctx)
		return err
	}
	*/

	fmt.Printf("BEGIN\n")
	tx, err := conn.Begin(ctx)
	if err != nil {
		tx.Rollback(ctx)
		return err
	}

	fmt.Printf("READ account balances...\n")
	var alice1Balance float64
	err = tx.QueryRow(ctx,
		`SELECT balance FROM accounts where name = 'Alice1'`).Scan(&alice1Balance)
	if err != nil {
		tx.Rollback(ctx)
		return fmt.Errorf("QueryRow failed: %v\n", err)
	}

	fmt.Printf("READ account Alice1 balance = %f\n", alice1Balance)

	var alice2Balance float64 // CL_PAUSE_1
	err = tx.QueryRow(ctx,
		`SELECT balance FROM accounts where name = 'Alice2'`).Scan(&alice2Balance)
	if err != nil {
		tx.Rollback(ctx)
		return fmt.Errorf("QueryRow failed: %v\n", err)
	}
	fmt.Printf("READ account Alice2 balance = %f\n", alice2Balance)

	// Now repeat the read... it is not repeatable so the total will be different

	fmt.Printf("Total account balance = %f\n", alice1Balance+alice2Balance)

	fmt.Printf("RE-READ accounts...\n")
	err = tx.QueryRow(ctx,
		`SELECT balance FROM accounts where name = 'Alice1'`).Scan(&alice1Balance)
	if err != nil {
		tx.Rollback(ctx)
		return fmt.Errorf("QueryRow failed: %v\n", err)
	}

	fmt.Printf("Alice1 account balance now: %f\n", alice1Balance)

	err = tx.QueryRow(ctx,
		`SELECT balance FROM accounts where name = 'Alice2'`).Scan(&alice2Balance)
	if err != nil {
		tx.Rollback(ctx)
		return fmt.Errorf("QueryRow failed: %v\n", err)
	}
	fmt.Printf("Alice2 account balance now: %f\n", alice2Balance)

	fmt.Printf("Total account balance now = %f\n", alice1Balance+alice2Balance)

	fmt.Printf("COMMIT\n")
	err = tx.Commit(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("Done\n")
	return nil
}

// bob performs a write which can cause a read skew for alice. Why is bob updating alice's accounts? IDK man maybe he's
// her financial advisor. Anyway he's transferring money from account Alice1 to Alice2.
//
func bob(conn *pgx.Conn) error {
	ctx := context.Background()

	// create transaction
	fmt.Printf("BEGIN\n")
	tx, err := conn.Begin(ctx)
	if err != nil {
		tx.Rollback(ctx)
		return err
	}

	fmt.Printf("UPDATE account Alice1: balance - 100\n")
	_, err = tx.Exec(ctx,
		`UPDATE accounts SET balance = balance - 100 where name = 'Alice1'`)
	if err != nil {
		tx.Rollback(ctx)
		return fmt.Errorf("Exec failed: %v\n", err)
	}

	fmt.Printf("UPDATE account Alice2: balance + 100\n")
	_, err = tx.Exec(ctx,
		`UPDATE accounts SET balance = balance + 100 where name = 'Alice2'`)
	if err != nil {
		tx.Rollback(ctx)
		return fmt.Errorf("Exec failed: %v\n", err)
	}

	fmt.Printf("COMMIT\n")
	err = tx.Commit(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("Done\n")
	return nil
}

func main() {
	abFlag := flag.String("user", "alice", "User: 'alice' or 'bob'")
	flag.Parse()
	if abFlag == nil {
		fmt.Fprintf(os.Stderr, "user flag is required\n")
		os.Exit(1)
	}
	if *abFlag != "alice" && *abFlag != "bob" {
		fmt.Fprintf(os.Stderr, "user flag must be one of 'alice' or 'bob'\n")
		os.Exit(1)
	}

	// DATABASE_URL env var: "postgres://postgres:pass@localhost:5433/postgres"
	conn, err := pgx.Connect(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(context.Background())

	if *abFlag == "alice" {
		err = alice(conn)
	} else if *abFlag == "bob" {
		err = bob(conn)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}

	os.Exit(1)
}
