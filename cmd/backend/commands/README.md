# GoLeapAI CLI Commands

This directory contains all the CLI command implementations for GoLeapAI.

## Structure

```
commands/
├── serve.go      - Start gateway server
├── providers.go  - Provider management commands
├── stats.go      - Statistics commands
├── config.go     - Configuration management
├── migrate.go    - Database migration commands
└── doctor.go     - Health diagnostics
```

## Command Overview

### serve
Start the GoLeapAI gateway server with all features enabled.

**Features:**
- Development mode with pretty logging
- Auto-migration support
- Graceful shutdown
- HTTP/3 support

### providers
Manage LLM provider configurations.

**Subcommands:**
- `list` - List all providers
- `add` - Add a new provider manually
- `remove` - Remove a provider
- `test` - Test provider connectivity
- `sync` - Sync from auto-discovery

### stats
View and manage statistics.

**Subcommands:**
- `show` - Display aggregated statistics
- `export` - Export to CSV/JSON
- `reset` - Reset statistics data

### config
Configuration file management.

**Subcommands:**
- `show` - Display current configuration
- `validate` - Validate configuration file
- `generate` - Generate template configuration

### migrate
Database migration management.

**Subcommands:**
- `up` - Run pending migrations
- `down` - Rollback migrations
- `reset` - Reset database (destructive)
- `seed` - Seed initial data
- `status` - Show migration status

### doctor
System health diagnostics.

**Checks:**
- Database connectivity
- Redis availability (optional)
- Provider health status
- System resources

## Development

### Adding a New Command

1. Create a new file in this directory (e.g., `mycommand.go`)
2. Define the command using Cobra:

```go
package commands

import "github.com/spf13/cobra"

var MyCmd = &cobra.Command{
    Use:   "mycommand",
    Short: "Brief description",
    Long:  "Detailed description",
    RunE:  runMyCommand,
}

func init() {
    // Add flags
    MyCmd.Flags().StringVar(&myVar, "flag", "", "description")
}

func runMyCommand(cmd *cobra.Command, args []string) error {
    // Implementation
    return nil
}
```

3. Add it to `main.go`:

```go
rootCmd.AddCommand(commands.MyCmd)
```

### Shared Utilities

Common functions used across commands:

- `initDB(cmd)` - Initialize database connection
- `printJSON(data)` - Pretty-print JSON output
- `formatTimeSince(t)` - Format time duration
- `printProvidersTable(providers)` - Display providers in table format

## Testing

Test commands manually:

```bash
# Build first
go build -o bin/goleapai ./cmd/backend

# Test commands
./bin/goleapai --help
./bin/goleapai serve --help
./bin/goleapai providers list --help
```

## Best Practices

1. **Error Handling**: Always return descriptive errors
2. **User Feedback**: Use clear success/failure messages
3. **Flags**: Follow Cobra conventions for flag naming
4. **Documentation**: Add examples to command help text
5. **Validation**: Validate inputs before processing
6. **Destructive Operations**: Require confirmation flags

## Examples

See `/docs/CLI_COMMANDS.md` for comprehensive usage examples.
