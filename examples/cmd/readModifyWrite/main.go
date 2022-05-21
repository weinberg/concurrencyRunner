package main

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v4"
	"os"
)

// readModifyWrite performs a read, modify and write which is susceptible to the "lost update" concurrency scenario.
//
// The code implements:
// - read count of email from database
// - increment (in code)
// - write it back out to the database
//
// If during the transaction, another process begins the same operation, one of the operation's writes will be lost.
func readModifyWrite(conn *pgx.Conn) error {
	const userId int = 1
	var unreadCount int
	ctx := context.Background()

	// create transaction - using repeatable read will avoid this bug by automatically catching this situation
	// and aborting the first transaction.
	fmt.Printf("BEGIN ISOLATION LEVEL REPEATABLE READ\n")
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{
		IsoLevel: pgx.RepeatableRead,
	})
	if err != nil {
		tx.Rollback(ctx)
		return err
	}

	// create transaction without repeatable read
	/*
		fmt.Printf("BEGIN\n")
		tx, err := conn.Begin(ctx)
		if err != nil {
			tx.Rollback(ctx)
			return err
		}
	*/

	// also this bug can be avoided by using SELECT FOR UPDATE

	err = tx.QueryRow(ctx,
		`SELECT unread FROM user_email_stats where user_id = $1
	`, userId).Scan(&unreadCount)
	if err != nil {
		tx.Rollback(ctx)
		return fmt.Errorf("QueryRow failed: %v\n", err)
	}
	fmt.Printf("READ: unreadCount = %d\n", unreadCount)

	unreadCount++
	fmt.Printf("MODIFY: adding 1 to unreadCount, now it is %d\n", unreadCount)

	fmt.Printf("WRITE: unreadCount = %d\n", unreadCount) // CL_PAUSE_1
	_, err = tx.Exec(ctx, `UPDATE user_email_stats set unread = $1 where user_id = $2`, unreadCount, userId)
	if err != nil {
		tx.Rollback(ctx)
		fmt.Printf("ERROR - ROLLBACK\n")
		return fmt.Errorf("Exec failed: %v\n", err)
	}

	fmt.Printf("COMMIT\n") // CL_PAUSE_2
	err = tx.Commit(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("Done\n") // CL_PAUSE_3
	return nil
}

func main() {
	// DATABASE_URL env var: "postgres://postgres:pass@localhost:5433/postgres"
	conn, err := pgx.Connect(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(context.Background())

	err = readModifyWrite(conn)

	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}

	os.Exit(1)
}
