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
	fmt.Printf("Role: %s\n", config.Role)
}

func ExampleLoad_workingSet() {
	// Create a temporary config file for demonstration
	dir, _ := os.MkdirTemp("", "example")
	defer os.RemoveAll(dir)

	configPath := filepath.Join(dir, "machine.yaml")
	content := `name: laptop
role: working-set
storage_host: storage.example.com
`
	os.WriteFile(configPath, []byte(content), 0644)

	// Load the config
	config, err := machine.Load(configPath)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Machine: %s\n", config.Name)
	fmt.Printf("Role: %s\n", config.Role)
	fmt.Printf("Storage Host: %s\n", config.StorageHost)
	// Output:
	// Machine: laptop
	// Role: working-set
	// Storage Host: storage.example.com
}

func ExampleMachineConfig_Validate() {
	// Valid storage config
	storageConfig := &machine.MachineConfig{
		Name: "storage-server",
		Role: machine.RoleStorage,
	}

	if err := storageConfig.Validate(); err != nil {
		fmt.Printf("Storage config error: %v\n", err)
	} else {
		fmt.Println("Storage config is valid")
	}

	// Valid working-set config
	workingSetConfig := &machine.MachineConfig{
		Name:        "laptop",
		Role:        machine.RoleWorkingSet,
		StorageHost: "storage.example.com",
	}

	if err := workingSetConfig.Validate(); err != nil {
		fmt.Printf("Working-set config error: %v\n", err)
	} else {
		fmt.Println("Working-set config is valid")
	}

	// Invalid config (missing storage_host for working-set)
	invalidConfig := &machine.MachineConfig{
		Name: "laptop",
		Role: machine.RoleWorkingSet,
	}

	if err := invalidConfig.Validate(); err != nil {
		fmt.Printf("Invalid config error: %v\n", err)
	}

	// Output:
	// Storage config is valid
	// Working-set config is valid
	// Invalid config error: storage_host is required for working-set role
}

func ExampleRole_IsValid() {
	validRoles := []machine.Role{
		machine.RoleStorage,
		machine.RoleWorkingSet,
	}

	for _, role := range validRoles {
		fmt.Printf("%s is valid: %v\n", role, role.IsValid())
	}

	invalidRole := machine.Role("invalid")
	fmt.Printf("%s is valid: %v\n", invalidRole, invalidRole.IsValid())

	// Output:
	// storage is valid: true
	// working-set is valid: true
	// invalid is valid: false
}
