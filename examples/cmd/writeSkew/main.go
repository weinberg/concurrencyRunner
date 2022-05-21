package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/jackc/pgx/v4"
	"os"
)

// An employee scheduling system requires at least one employee to be working at all times.
// Employees can give up their shift as long as at least one other employee is still working that shift.
// Write skew can happen in this scenario. Using repeatable read will not fix this.
//
// This is an example of "serialization anomaly" which postgres defines as:
// The result of successfully committing a group of transactions is inconsistent with all
// possible orderings of running those transactions one at a time.
func removeEmployeeShift(conn *pgx.Conn, shift_id int, employee_id int) error {
	ctx := context.Background()

	// create transaction
	// using repeatable read will not fix this bug but serializeable will:
	/*
		fmt.Printf("BEGIN ISOLATION LEVEL SERIALIZABLE\n")
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{
			IsoLevel: pgx.Serializable,
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

	fmt.Printf("READ number of employees working shift %d...\n", shift_id)
	var count int
	err = tx.QueryRow(ctx,
		`SELECT count(*) FROM employee_shift WHERE shift_id = $1`, shift_id).Scan(&count)
	if err != nil {
		tx.Rollback(ctx)
		return fmt.Errorf("QueryRow failed: %v\n", err)
	}

	fmt.Printf("READ count of employees on shift %d = %d\n", shift_id, count)
	if count > 1 {
		fmt.Printf("COMPARE count is > 1. OK to remove employee from shift.\n")
	} else {
		fmt.Printf("COMPARE count is <= 1. Not OK to remove employee from shift. RETURNING\n")
		return nil
	}

	fmt.Printf("DELETE employee %d from shift %d\n", employee_id, shift_id) // CL_PAUSE_1
	_, err = tx.Exec(ctx,
		`DELETE FROM employee_shift WHERE employee_id = $1 AND shift_id = $2`, employee_id, shift_id)
	if err != nil {
		tx.Rollback(ctx)
		return fmt.Errorf("Exec failed: %v\n", err)
	}

	fmt.Printf("COMMIT\n")
	err = tx.Commit(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("DONE\n")
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
		err = removeEmployeeShift(conn, 1234, 1)
	} else if *abFlag == "bob" {
		err = removeEmployeeShift(conn, 1234, 2)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}

	os.Exit(1)
}
