// genvapid generates a VAPID key pair for Web Push notifications.
// Run once and persist the output in your environment variables.
//
//	go run ./cmd/genvapid
package main

import (
	"fmt"
	"os"

	"webapp/internal/push"
)

func main() {
	priv, pub, err := push.GenerateVAPIDKeys()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("# Add these to your .env.local (or Render environment variables):")
	fmt.Printf("VAPID_PUBLIC_KEY=%s\n", pub)
	fmt.Printf("VAPID_PRIVATE_KEY=%s\n", priv)
	fmt.Println("VAPID_SUBJECT=mailto:admin@agronomia.app")
}
