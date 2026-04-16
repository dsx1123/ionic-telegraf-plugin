# Telegraf Input Plugin for AMD Pensando AINIC (nicctl)

A [Telegraf](https://github.com/influxdata/telegraf) external input plugin that collects runtime statistics from AMD Pensando AINIC cards via the `nicctl` CLI tool. It runs configurable commands at user-defined intervals and generically parses their JSON output into Telegraf metrics.

## How It Works

The plugin runs as an [execd](https://github.com/influxdata/telegraf/tree/master/plugins/inputs/execd) external plugin. It executes `nicctl` commands, flattens the JSON output into key-value fields, and emits InfluxDB line protocol to stdout.

**Command groups** allow different sets of commands to run at independent intervals:

```toml
[[inputs.nicctl]]
  [[inputs.nicctl.command_group]]
    interval = "5s"
    commands = [
      "nicctl show port statistics",
      "nicctl show lif statistics",
    ]

  [[inputs.nicctl.command_group]]
    interval = "30s"
    commands = [
      "nicctl show card statistics packet-buffer",
    ]
```

`sudo` and `--json` are automatically added to commands if not already present.

## Building

```sh
make build
```

Produces `bin/ionic-telegraf-plugin`.

## Testing

```sh
# Unit tests
make test

# Integration tests (requires SSH access to a host with nicctl)
INTEGRATION_HOST=10.9.10.145 make integration
```

## Installation

1. Build the binary and copy it to the target host:
   ```sh
   scp bin/ionic-telegraf-plugin target:/usr/local/bin/
   ```

2. Copy the plugin config:
   ```sh
   scp plugin.conf target:/etc/telegraf/plugin.conf
   ```

3. Add the execd input to your `telegraf.conf`:
   ```toml
   [[inputs.execd]]
     command = ["/usr/local/bin/ionic-telegraf-plugin", "-config", "/etc/telegraf/plugin.conf", "-poll_interval", "1s"]
     signal = "none"
   ```

4. Restart Telegraf.

## Configuration

### Plugin Config (`plugin.conf`)

| Field | Description |
|---|---|
| `interval` | Polling interval for the command group (e.g., `"5s"`, `"1m"`). Minimum `1s`. |
| `commands` | List of `nicctl` commands to run. `sudo` and `--json` are automatically added if not present. |
| `measurement_overrides` | Optional map of command string to custom measurement name. |

### Measurement Names

Measurement names are derived automatically from the command string by stripping `sudo`, `nicctl`, `show`, `--json`, and any flags, then joining remaining tokens with `_` and prefixing with `nicctl_`.

| Command | Measurement |
|---|---|
| `nicctl show port statistics` | `nicctl_port_statistics` |
| `nicctl show lif statistics` | `nicctl_lif_statistics` |
| `nicctl show card statistics packet-buffer` | `nicctl_card_statistics_packet_buffer` |

Override with `measurement_overrides`:
```toml
[inputs.nicctl.command_group.measurement_overrides]
  "nicctl show port statistics" = "my_port_stats"
```

### JSON Flattening

Nested JSON is flattened with `_` separators. Arrays use integer indices (`key_0`, `key_1`). Numbers without fractional parts are stored as integers. Nulls are dropped.

### Tags

Each metric includes a `command` tag with the full command string.

## Project Structure

```
cmd/main.go                              # execd shim entrypoint
plugins/inputs/nicctl/
  nicctl.go                              # Core plugin (Init, Gather, registration)
  nicctl_test.go                         # Unit tests with mock runner
  flatten.go                             # Generic JSON flattener
  flatten_test.go                        # Flatten unit tests
  measurement.go                         # Measurement name derivation
  measurement_test.go                    # Measurement name tests
internal/exec/runner.go                  # Command execution interface
tests/integration/integration_test.go    # SSH-based integration tests
plugin.conf                              # Plugin-specific config
telegraf.conf                            # Example Telegraf execd wrapper config
```

## License

See LICENSE file.
