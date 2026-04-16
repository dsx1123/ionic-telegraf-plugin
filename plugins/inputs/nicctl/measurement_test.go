package nicctl

import "testing"

func TestDeriveMeasurement(t *testing.T) {
	tests := []struct {
		command  string
		expected string
	}{
		{
			command:  "sudo nicctl show port statistics --json",
			expected: "nicctl_port_statistics",
		},
		{
			command:  "sudo nicctl show card statistics packet-buffer --json",
			expected: "nicctl_card_statistics_packet_buffer",
		},
		{
			command:  "nicctl show port statistics --json",
			expected: "nicctl_port_statistics",
		},
		{
			command:  "sudo nicctl show lif statistics --json",
			expected: "nicctl_lif_statistics",
		},
		{
			command:  "sudo nicctl show port --json --verbose",
			expected: "nicctl_port",
		},
		{
			command:  "sudo nicctl show --json",
			expected: "nicctl",
		},
		{
			command:  "sudo nicctl show Port-Stats --json",
			expected: "nicctl_port_stats",
		},
		{
			command:  "nicctl show port statistics -v --json",
			expected: "nicctl_port_statistics",
		},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			got := DeriveMeasurement(tt.command)
			if got != tt.expected {
				t.Errorf("DeriveMeasurement(%q) = %q, want %q", tt.command, got, tt.expected)
			}
		})
	}
}
