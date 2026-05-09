
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

package main

import (
	"flag"
	"fmt"
	"os"

	exerciseone "github.com/muhammadsziyad/redis-exercise-one.git"
)

// main initializes and starts the application execution.
func main() {
	sourceAddr := flag.String("source", "localhost:12000", "source-db Redis Address (host:port)")
	replicaAddr := flag.String("replica", "localhost:12001", "replica-db Redis Address (host:port)")
	flag.Parse()

	fmt.Printf("Source DB  : %s\n", *sourceAddr)
	fmt.Printf("Replica DB : %s\n\n", *replicaAddr)

	exerciseone.Run(*sourceAddr, *replicaAddr);

	os.Exit(0)
}
