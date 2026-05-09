// ============================================================
// PS Consulting Engineer Technical Challenge
// Author      : Muhammad S Ziyad
// Description : Exercise Number 1
// Usage       : go run main/main.go
//
// Notes:
// - I Designed to run from command line
// - Replace configuration values as needed
// ============================================================

// Package exerciseone demonstrates Redis database synchronization using
// Sorted Sets to insert values 1-100 into source-db and read them
// in reverse order from replica-db.
package exerciseone

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// SortedSetKey is the Redis key used to store the sequence of numbers
const (
	SortedSetKey = "numbers:sequence"
)

// Client wraps Redis clients for source and replica databases
type Client struct {
	Source  *redis.Client
	Replica *redis.Client
}

// NewClient creates new Redis clients for source and replica endpoints
func NewClient(sourceAddr, replicaAddr string) *Client {
	return &Client{
		Source: redis.NewClient(&redis.Options{
			Addr:        sourceAddr,
			DialTimeout: 5 * time.Second,
			ReadTimeout: 5 * time.Second,
		}),
		Replica: redis.NewClient(&redis.Options{
			Addr:        replicaAddr,
			DialTimeout: 5 * time.Second,
			ReadTimeout: 5 * time.Second,
		}),
	}
}

// Close closes both Redis client connections
func (c *Client) Close() {
	c.Source.Close()
	c.Replica.Close()
}

// InsertValues inserts integers 1 to 100 into the source-db using a Redis Sorted Set.
// Each number is stored as both the member and the score, enabling native
// reverse-order retrieval via ZRangeArgs with Rev: true (replaces deprecated ZRevRange).
func InsertValues(ctx context.Context, client *redis.Client) error {
	pipe := client.Pipeline()

	// Remove any existing data at this key
	pipe.Del(ctx, SortedSetKey)

	// Build all 100 members as Sorted Set entries: score = value = member
	members := make([]redis.Z, 100)
	for i := 1; i <= 100; i++ {
		members[i-1] = redis.Z{
			Score:  float64(i),
			Member: strconv.Itoa(i),
		}
	}
	pipe.ZAdd(ctx, SortedSetKey, members...)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to insert values into source-db: %w", err)
	}
	return nil
}

// ReadReverse reads all values from the Sorted Set in descending order (100 → 1).
// Uses ZRangeArgs with Rev: true — the modern replacement for the deprecated ZRevRange.
// Equivalent Redis command: ZRANGE numbers:sequence 0 -1 REV
func ReadReverse(ctx context.Context, client *redis.Client) ([]string, error) {
	values, err := client.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:   SortedSetKey,
		Start: 0,
		Stop:  -1,
		Rev:   true,
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to read from replica-db: %w", err)
	}
	return values, nil
}

// VerifyReplication checks that replica-db has received the data from source-db
func VerifyReplication(ctx context.Context, source, replica *redis.Client) (bool, error) {
	srcCount, err := source.ZCard(ctx, SortedSetKey).Result()
	if err != nil {
		return false, fmt.Errorf("cannot read source cardinality: %w", err)
	}

	repCount, err := replica.ZCard(ctx, SortedSetKey).Result()
	if err != nil {
		return false, fmt.Errorf("cannot read replica cardinality: %w", err)
	}

	return srcCount == repCount && repCount == 100, nil
}

// Run executes the full Exercise 1 workflow
func Run(sourceAddr, replicaAddr string) {
	ctx := context.Background()

	fmt.Println("================================================================")
	fmt.Println("  Exercise 1: Building and Synchronizing Redis Databases")
	fmt.Println("================================================================")

	c := NewClient(sourceAddr, replicaAddr)
	defer c.Close()

	// Verify connectivity
	if err := c.Source.Ping(ctx).Err(); err != nil {
		log.Fatalf("[ERROR] Cannot connect to source-db at %s: %v", sourceAddr, err)
	}
	fmt.Printf("[OK] Connected to source-db at %s\n", sourceAddr)

	if err := c.Replica.Ping(ctx).Err(); err != nil {
		log.Fatalf("[ERROR] Cannot connect to replica-db at %s: %v", replicaAddr, err)
	}
	fmt.Printf("[OK] Connected to replica-db at %s\n", replicaAddr)

	// Step 1: Insert 1-100 into source-db
	fmt.Println("\n[Step 1] Inserting values 1-100 into source-db (Sorted Set)...")
	if err := InsertValues(ctx, c.Source); err != nil {
		log.Fatalf("[ERROR] %v", err)
	}

	count, _ := c.Source.ZCard(ctx, SortedSetKey).Result()
	fmt.Printf("[OK] Inserted %d values into source-db (key: '%s')\n", count, SortedSetKey)

	// Step 2: Wait for replication propagation
	fmt.Println("\n[Step 2] Waiting for replication to propagate (3 seconds)...")
	for i := 1; i <= 3; i++ {
		time.Sleep(1 * time.Second)
		ok, _ := VerifyReplication(ctx, c.Source, c.Replica)
		if ok {
			fmt.Printf("[OK] Replication confirmed after %d second(s)\n", i)
			break
		}
		if i == 3 {
			fmt.Println("[WARN] Replication may still be in progress, proceeding anyway...")
		}
	}

	// Step 3: Read in reverse order from replica-db
	fmt.Println("\n[Step 3] Reading values in reverse order from replica-db...")
	values, err := ReadReverse(ctx, c.Replica)
	if err != nil {
		log.Fatalf("[ERROR] %v", err)
	}

	fmt.Printf("[OK] Read %d values from replica-db\n", len(values))
	fmt.Println("\n--- Values (descending order: 100 → 1) ---")
	for i, v := range values {
		fmt.Print(v)
		if i < len(values)-1 {
			fmt.Print(", ")
		}
		// Line break every 10 values for readability
		if (i+1)%10 == 0 {
			fmt.Println()
		}
	}
	fmt.Println("\n------------------------------------------")
	fmt.Println("\n[DONE] Exercise 1 completed successfully!")
}
