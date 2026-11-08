// Command verify checks generated update manifests without installing them.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/DeliciousBuding/rclonego/internal/updates"
)

func main() {
	directory := flag.String("directory", "", "directory of MyGo update artifacts")
	configPath := flag.String("config", "mygo.json", "public MyGo configuration")
	required := flag.Bool("required", false, "require at least one signed manifest")
	flag.Parse()
	raw, err := os.ReadFile(*configPath)
	if err != nil {
		fail(err)
	}
	var config struct {
		Version string `json:"version"`
		Updates *struct {
			PublicKey string `json:"publicKey"`
		} `json:"updates"`
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		fail(err)
	}
	if config.Updates == nil {
		fail(fmt.Errorf("signed artifacts need updates.publicKey in %s", *configPath))
	}
	count, err := updates.VerifyDirectory(*directory, config.Updates.PublicKey, config.Version, *required)
	if err != nil {
		fail(err)
	}
	fmt.Printf("Verified %d official MyGo update manifest(s).\n", count)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
