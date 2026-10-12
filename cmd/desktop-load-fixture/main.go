package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"hostlight/internal/loadfixture"
	"os"
	"os/signal"
	"path/filepath"
	"time"
)

func main() {
	dir := flag.String("dir", "", "Empty absolute directory for disposable data")
	duration := flag.Duration("duration", time.Hour, "Maximum fixture lifetime")
	flag.Parse()
	if *duration < time.Minute || *duration > 24*time.Hour {
		fmt.Fprintln(os.Stderr, "Duration must be 1 minute–24 hours")
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, *duration)
	defer cancel()
	fixture, err := loadfixture.Start(ctx, *dir, 200)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer fixture.Close()
	ready, err := json.Marshal(fixture)
	if err == nil {
		err = os.WriteFile(filepath.Join(*dir, "ready.json"), ready, 0600)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		fixture.Close()
		os.Exit(1)
	}
	fmt.Println(string(ready))
	<-ctx.Done()
}
