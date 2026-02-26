package machine_test

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/PoeAudits/overlord/internal/machine"
)

func ExampleLoad() {
	// Load machine config from default location
	config, err := machine.Load(machine.DefaultConfigPath)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Machine: %s\n", config.Name)
	fmt.Printf("Syncthing configured: %t\n", config.Syncthing.DeviceID != "")
}

func ExampleLoad_syncthing() {
	// Create a temporary config file for demonstration
	dir, _ := os.MkdirTemp("", "example")
	defer os.RemoveAll(dir)

	configPath := filepath.Join(dir, "machine.yaml")
	content := `name: laptop
syncthing:
  device_id: "XXXX-XXXX-XXXX-XXXX"
  api_url: "http://localhost:8384"
  api_key: "example-key"
  address: "tcp://100.100.100.100:22000"
`
	_ = os.WriteFile(configPath, []byte(content), 0o644)

	// Load the config
	config, err := machine.Load(configPath)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Machine: %s\n", config.Name)
	fmt.Printf("Syncthing device: %s\n", config.Syncthing.DeviceID)
	// Output:
	// Machine: laptop
	// Syncthing device: XXXX-XXXX-XXXX-XXXX
}

func ExampleMachineConfig_Validate() {
	// Valid minimal config
	minimal := &machine.MachineConfig{
		Name: "desktop",
	}

	if err := minimal.Validate(); err != nil {
		fmt.Printf("Minimal config error: %v\n", err)
	} else {
		fmt.Println("Minimal config is valid")
	}

	// Valid Syncthing config
	configured := &machine.MachineConfig{
		Name: "vps",
		Syncthing: machine.SyncthingConfig{
			DeviceID: "XXXX-XXXX-XXXX-XXXX",
			APIURL:   "http://localhost:8384",
			APIKey:   "key",
			Address:  "tcp://100.100.100.100:22000",
		},
	}

	if err := configured.Validate(); err != nil {
		fmt.Printf("Configured config error: %v\n", err)
	} else {
		fmt.Println("Configured config is valid")
	}

	// Invalid config (missing api_key)
	invalid := &machine.MachineConfig{
		Name: "broken",
		Syncthing: machine.SyncthingConfig{
			DeviceID: "XXXX-XXXX-XXXX-XXXX",
			APIURL:   "http://localhost:8384",
			Address:  "tcp://100.100.100.100:22000",
		},
	}

	if err := invalid.Validate(); err != nil {
		fmt.Printf("Invalid config error: %v\n", err)
	}

	// Output:
	// Minimal config is valid
	// Configured config is valid
	// Invalid config error: syncthing config requires device_id, api_url, and api_key
}
