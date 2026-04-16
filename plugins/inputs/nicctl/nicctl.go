package nicctl

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"
	iexec "github.com/pensando/ionic-telegraf-plugin/internal/exec"
)

type CommandGroup struct {
	Interval             string            `toml:"interval"`
	Commands             []string          `toml:"commands"`
	MeasurementOverrides map[string]string `toml:"measurement_overrides"`

	parsedInterval time.Duration
	lastRun        map[string]time.Time
}

type NicctlPlugin struct {
	CommandGroups []*CommandGroup `toml:"command_group"`
	runner        iexec.Runner
	nowFunc       func() time.Time
	mu            sync.Mutex
	Log           telegraf.Logger `toml:"-"`
}

func (n *NicctlPlugin) SampleConfig() string {
	return `
  ## Command groups define sets of nicctl commands with independent polling intervals.
  ## "sudo" and "--json" are automatically added if not present.
  [[inputs.nicctl.command_group]]
    interval = "5s"
    commands = [
      "nicctl show port statistics",
      "nicctl show lif statistics",
    ]
    # Optional: override derived measurement names per command.
    # [inputs.nicctl.command_group.measurement_overrides]
    #   "nicctl show port statistics" = "my_port_stats"

  [[inputs.nicctl.command_group]]
    interval = "30s"
    commands = [
      "nicctl show card statistics packet-buffer",
    ]
`
}

func (n *NicctlPlugin) Description() string {
	return "Collects metrics from AMD Pensando AINIC cards via nicctl CLI"
}

func (n *NicctlPlugin) Init() error {
	if len(n.CommandGroups) == 0 {
		return fmt.Errorf("at least one command_group is required")
	}

	for i, group := range n.CommandGroups {
		if len(group.Commands) == 0 {
			return fmt.Errorf("command_group[%d]: commands list is empty", i)
		}

		d, err := time.ParseDuration(group.Interval)
		if err != nil {
			return fmt.Errorf("command_group[%d]: invalid interval %q: %w", i, group.Interval, err)
		}
		if d < time.Second {
			return fmt.Errorf("command_group[%d]: interval must be >= 1s, got %s", i, d)
		}
		group.parsedInterval = d
		group.lastRun = make(map[string]time.Time)

		for j, cmd := range group.Commands {
			group.Commands[j] = normalizeCommand(cmd)
		}

		if group.MeasurementOverrides != nil {
			normalized := make(map[string]string, len(group.MeasurementOverrides))
			for k, v := range group.MeasurementOverrides {
				normalized[normalizeCommand(k)] = v
			}
			group.MeasurementOverrides = normalized
		}
	}

	if n.runner == nil {
		n.runner = &iexec.DefaultRunner{}
	}
	if n.nowFunc == nil {
		n.nowFunc = time.Now
	}

	return nil
}

func (n *NicctlPlugin) Gather(acc telegraf.Accumulator) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	now := n.nowFunc()

	for _, group := range n.CommandGroups {
		for _, cmd := range group.Commands {
			last, exists := group.lastRun[cmd]
			if exists && now.Sub(last) < group.parsedInterval {
				continue
			}

			out, err := n.runner.Run(cmd)
			if err != nil {
				acc.AddError(fmt.Errorf("command %q: %w", cmd, err))
				continue
			}

			fields, err := FlattenJSON(out)
			if err != nil {
				acc.AddError(fmt.Errorf("command %q: %w", cmd, err))
				continue
			}

			if len(fields) == 0 {
				continue
			}

			measurement := DeriveMeasurement(cmd)
			if group.MeasurementOverrides != nil {
				if override, ok := group.MeasurementOverrides[cmd]; ok {
					measurement = override
				}
			}

			tags := map[string]string{
				"command": cmd,
			}

			acc.AddFields(measurement, fields, tags, now)
			group.lastRun[cmd] = now
		}
	}

	return nil
}

// normalizeCommand ensures the command has "sudo" prefix and "--json" suffix.
func normalizeCommand(cmd string) string {
	if !strings.HasPrefix(cmd, "sudo ") {
		cmd = "sudo " + cmd
	}
	if !strings.HasSuffix(cmd, " --json") {
		cmd = cmd + " --json"
	}
	return cmd
}

func init() {
	inputs.Add("nicctl", func() telegraf.Input {
		return &NicctlPlugin{}
	})
}
