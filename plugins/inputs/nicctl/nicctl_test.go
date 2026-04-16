package nicctl

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/testutil"
)

// MockRunner implements exec.Runner for testing.
type MockRunner struct {
	mu       sync.Mutex
	outputs  map[string][]byte
	errors   map[string]error
	calls    []string
}

func NewMockRunner() *MockRunner {
	return &MockRunner{
		outputs: make(map[string][]byte),
		errors:  make(map[string]error),
	}
}

func (m *MockRunner) SetOutput(command string, output []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.outputs[command] = output
}

func (m *MockRunner) SetError(command string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors[command] = err
}

func (m *MockRunner) Run(command string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, command)
	if err, ok := m.errors[command]; ok {
		return nil, err
	}
	if out, ok := m.outputs[command]; ok {
		return out, nil
	}
	return nil, fmt.Errorf("no mock output for command %q", command)
}

func (m *MockRunner) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

func (m *MockRunner) Calls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := make([]string, len(m.calls))
	copy(c, m.calls)
	return c
}

func TestInit_NoGroups(t *testing.T) {
	p := &NicctlPlugin{}
	err := p.Init()
	if err == nil {
		t.Fatal("expected error for no command groups")
	}
}

func TestInit_BadInterval(t *testing.T) {
	p := &NicctlPlugin{
		CommandGroups: []*CommandGroup{
			{Interval: "notaduration", Commands: []string{"cmd"}},
		},
	}
	err := p.Init()
	if err == nil {
		t.Fatal("expected error for bad interval")
	}
}

func TestInit_IntervalTooShort(t *testing.T) {
	p := &NicctlPlugin{
		CommandGroups: []*CommandGroup{
			{Interval: "500ms", Commands: []string{"cmd"}},
		},
	}
	err := p.Init()
	if err == nil {
		t.Fatal("expected error for interval < 1s")
	}
}

func TestInit_NoCommands(t *testing.T) {
	p := &NicctlPlugin{
		CommandGroups: []*CommandGroup{
			{Interval: "5s", Commands: []string{}},
		},
	}
	err := p.Init()
	if err == nil {
		t.Fatal("expected error for empty commands")
	}
}

func TestInit_Valid(t *testing.T) {
	p := &NicctlPlugin{
		CommandGroups: []*CommandGroup{
			{Interval: "5s", Commands: []string{"cmd1"}},
		},
	}
	err := p.Init()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInit_NormalizeCommands(t *testing.T) {
	p := &NicctlPlugin{
		CommandGroups: []*CommandGroup{
			{Interval: "5s", Commands: []string{
				"nicctl show port statistics",
				"sudo nicctl show lif statistics --json",
				"sudo nicctl show card statistics",
				"nicctl show system --json",
			}},
		},
	}
	if err := p.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	cmds := p.CommandGroups[0].Commands
	expected := []string{
		"sudo nicctl show port statistics --json",
		"sudo nicctl show lif statistics --json",
		"sudo nicctl show card statistics --json",
		"sudo nicctl show system --json",
	}
	for i, want := range expected {
		if cmds[i] != want {
			t.Errorf("command[%d]: got %q, want %q", i, cmds[i], want)
		}
	}
}

func TestGather_FirstRun(t *testing.T) {
	mock := NewMockRunner()
	// Init will prepend sudo
	execCmd := "sudo nicctl show port statistics --json"
	mock.SetOutput(execCmd, []byte(`{"tx": 100, "rx": 200}`))

	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	p := &NicctlPlugin{
		CommandGroups: []*CommandGroup{
			{Interval: "5s", Commands: []string{"nicctl show port statistics"}},
		},
		runner:  mock,
		nowFunc: func() time.Time { return now },
	}
	if err := p.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	acc := &testutil.Accumulator{}
	if err := p.Gather(acc); err != nil {
		t.Fatalf("Gather failed: %v", err)
	}

	if len(acc.Metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(acc.Metrics))
	}

	m := acc.Metrics[0]
	if m.Measurement != "nicctl_port_statistics" {
		t.Errorf("unexpected measurement: %s", m.Measurement)
	}
	if m.Fields["tx"] != int64(100) {
		t.Errorf("expected tx=100, got %v", m.Fields["tx"])
	}
	if m.Fields["rx"] != int64(200) {
		t.Errorf("expected rx=200, got %v", m.Fields["rx"])
	}
	if m.Tags["command"] != execCmd {
		t.Errorf("expected command tag %q, got %v", execCmd, m.Tags["command"])
	}
}

func TestGather_SkipsBeforeInterval(t *testing.T) {
	mock := NewMockRunner()
	mock.SetOutput("sudo nicctl show port statistics --json", []byte(`{"tx": 100}`))

	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	p := &NicctlPlugin{
		CommandGroups: []*CommandGroup{
			{Interval: "5s", Commands: []string{"nicctl show port statistics"}},
		},
		runner:  mock,
		nowFunc: func() time.Time { return now },
	}
	if err := p.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	acc := &testutil.Accumulator{}
	_ = p.Gather(acc)
	if mock.CallCount() != 1 {
		t.Fatalf("expected 1 call, got %d", mock.CallCount())
	}

	// Advance 2s — still under 5s interval
	now = now.Add(2 * time.Second)
	acc = &testutil.Accumulator{}
	_ = p.Gather(acc)
	if mock.CallCount() != 1 {
		t.Fatalf("expected still 1 call (skipped), got %d", mock.CallCount())
	}
	if len(acc.Metrics) != 0 {
		t.Fatalf("expected 0 metrics on skip, got %d", len(acc.Metrics))
	}
}

func TestGather_RunsAfterInterval(t *testing.T) {
	mock := NewMockRunner()
	mock.SetOutput("sudo nicctl show port statistics --json", []byte(`{"tx": 100}`))

	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	p := &NicctlPlugin{
		CommandGroups: []*CommandGroup{
			{Interval: "5s", Commands: []string{"nicctl show port statistics"}},
		},
		runner:  mock,
		nowFunc: func() time.Time { return now },
	}
	if err := p.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	acc := &testutil.Accumulator{}
	_ = p.Gather(acc)

	// Advance past interval
	now = now.Add(6 * time.Second)
	acc = &testutil.Accumulator{}
	_ = p.Gather(acc)
	if mock.CallCount() != 2 {
		t.Fatalf("expected 2 calls, got %d", mock.CallCount())
	}
	if len(acc.Metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(acc.Metrics))
	}
}

func TestGather_CommandFailureDoesNotCrash(t *testing.T) {
	mock := NewMockRunner()
	mock.SetError("sudo nicctl show port statistics --json", fmt.Errorf("command failed"))
	mock.SetOutput("sudo nicctl show lif statistics --json", []byte(`{"active": 5}`))

	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	p := &NicctlPlugin{
		CommandGroups: []*CommandGroup{
			{Interval: "5s", Commands: []string{
				"nicctl show port statistics",
				"nicctl show lif statistics",
			}},
		},
		runner:  mock,
		nowFunc: func() time.Time { return now },
	}
	if err := p.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	acc := &testutil.Accumulator{}
	_ = p.Gather(acc)

	if len(acc.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(acc.Errors))
	}
	if len(acc.Metrics) != 1 {
		t.Fatalf("expected 1 metric (from cmd2), got %d", len(acc.Metrics))
	}
}

func TestGather_InvalidJSONDoesNotCrash(t *testing.T) {
	mock := NewMockRunner()
	mock.SetOutput("sudo nicctl show port statistics --json", []byte(`not valid json`))

	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	p := &NicctlPlugin{
		CommandGroups: []*CommandGroup{
			{Interval: "5s", Commands: []string{"nicctl show port statistics"}},
		},
		runner:  mock,
		nowFunc: func() time.Time { return now },
	}
	if err := p.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	acc := &testutil.Accumulator{}
	_ = p.Gather(acc)

	if len(acc.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(acc.Errors))
	}
	if len(acc.Metrics) != 0 {
		t.Fatalf("expected 0 metrics, got %d", len(acc.Metrics))
	}
}

func TestGather_MeasurementOverride(t *testing.T) {
	mock := NewMockRunner()
	mock.SetOutput("sudo nicctl show port statistics --json", []byte(`{"tx": 1}`))

	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	p := &NicctlPlugin{
		CommandGroups: []*CommandGroup{
			{
				Interval: "5s",
				Commands: []string{"nicctl show port statistics"},
				MeasurementOverrides: map[string]string{
					"nicctl show port statistics": "custom_port_stats",
				},
			},
		},
		runner:  mock,
		nowFunc: func() time.Time { return now },
	}
	if err := p.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	acc := &testutil.Accumulator{}
	_ = p.Gather(acc)

	if len(acc.Metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(acc.Metrics))
	}
	if acc.Metrics[0].Measurement != "custom_port_stats" {
		t.Errorf("expected custom_port_stats, got %s", acc.Metrics[0].Measurement)
	}
}

func TestGather_MultipleGroupsIndependent(t *testing.T) {
	mock := NewMockRunner()
	mock.SetOutput("sudo nicctl show port statistics --json", []byte(`{"tx": 1}`))
	mock.SetOutput("sudo nicctl show card statistics packet-buffer --json", []byte(`{"buf": 2}`))

	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	p := &NicctlPlugin{
		CommandGroups: []*CommandGroup{
			{Interval: "5s", Commands: []string{"nicctl show port statistics"}},
			{Interval: "30s", Commands: []string{"nicctl show card statistics packet-buffer"}},
		},
		runner:  mock,
		nowFunc: func() time.Time { return now },
	}
	if err := p.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// First run: both execute
	acc := &testutil.Accumulator{}
	_ = p.Gather(acc)
	if len(acc.Metrics) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(acc.Metrics))
	}

	// After 6s: only cmd1 (5s group) should re-run
	now = now.Add(6 * time.Second)
	acc = &testutil.Accumulator{}
	_ = p.Gather(acc)
	if len(acc.Metrics) != 1 {
		t.Fatalf("expected 1 metric (only 5s group), got %d", len(acc.Metrics))
	}
	if acc.Metrics[0].Measurement != "nicctl_port_statistics" {
		t.Errorf("expected nicctl_port_statistics, got %s", acc.Metrics[0].Measurement)
	}

	// After 31s total: both should run again
	now = now.Add(25 * time.Second) // 6+25 = 31s
	acc = &testutil.Accumulator{}
	_ = p.Gather(acc)
	if len(acc.Metrics) != 2 {
		t.Fatalf("expected 2 metrics (both groups), got %d", len(acc.Metrics))
	}
}

func TestGather_TagsIncludeCommand(t *testing.T) {
	mock := NewMockRunner()
	execCmd := "sudo nicctl show port statistics --json"
	mock.SetOutput(execCmd, []byte(`{"tx": 1}`))

	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	p := &NicctlPlugin{
		CommandGroups: []*CommandGroup{
			{Interval: "5s", Commands: []string{"nicctl show port statistics"}},
		},
		runner:  mock,
		nowFunc: func() time.Time { return now },
	}
	_ = p.Init()

	acc := &testutil.Accumulator{}
	_ = p.Gather(acc)

	if len(acc.Metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(acc.Metrics))
	}
	if acc.Metrics[0].Tags["command"] != execCmd {
		t.Errorf("expected command tag %q, got %q", execCmd, acc.Metrics[0].Tags["command"])
	}
}

func TestInit_EmptyInterval(t *testing.T) {
	p := &NicctlPlugin{
		CommandGroups: []*CommandGroup{
			{Interval: "", Commands: []string{"nicctl show port statistics"}},
		},
	}
	err := p.Init()
	if err == nil {
		t.Fatal("expected error for empty interval")
	}
}

func TestInit_NegativeInterval(t *testing.T) {
	p := &NicctlPlugin{
		CommandGroups: []*CommandGroup{
			{Interval: "-5s", Commands: []string{"nicctl show port statistics"}},
		},
	}
	err := p.Init()
	if err == nil {
		t.Fatal("expected error for negative interval")
	}
}

func TestInit_ZeroInterval(t *testing.T) {
	p := &NicctlPlugin{
		CommandGroups: []*CommandGroup{
			{Interval: "0s", Commands: []string{"nicctl show port statistics"}},
		},
	}
	err := p.Init()
	if err == nil {
		t.Fatal("expected error for zero interval")
	}
}

func TestInit_MultipleGroupsOneInvalid(t *testing.T) {
	p := &NicctlPlugin{
		CommandGroups: []*CommandGroup{
			{Interval: "5s", Commands: []string{"nicctl show port statistics"}},
			{Interval: "bad", Commands: []string{"nicctl show lif statistics"}},
		},
	}
	err := p.Init()
	if err == nil {
		t.Fatal("expected error when one group has bad interval")
	}
}

func TestGather_AllCommandsFail(t *testing.T) {
	mock := NewMockRunner()
	mock.SetError("sudo nicctl show port statistics --json", fmt.Errorf("port failed"))
	mock.SetError("sudo nicctl show lif statistics --json", fmt.Errorf("lif failed"))

	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	p := &NicctlPlugin{
		CommandGroups: []*CommandGroup{
			{Interval: "5s", Commands: []string{
				"nicctl show port statistics",
				"nicctl show lif statistics",
			}},
		},
		runner:  mock,
		nowFunc: func() time.Time { return now },
	}
	if err := p.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	acc := &testutil.Accumulator{}
	err := p.Gather(acc)
	if err != nil {
		t.Fatalf("Gather should not return error, got: %v", err)
	}
	if len(acc.Errors) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(acc.Errors))
	}
	if len(acc.Metrics) != 0 {
		t.Fatalf("expected 0 metrics, got %d", len(acc.Metrics))
	}
}

func TestGather_EmptyJSONObject(t *testing.T) {
	mock := NewMockRunner()
	mock.SetOutput("sudo nicctl show port statistics --json", []byte(`{}`))

	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	p := &NicctlPlugin{
		CommandGroups: []*CommandGroup{
			{Interval: "5s", Commands: []string{"nicctl show port statistics"}},
		},
		runner:  mock,
		nowFunc: func() time.Time { return now },
	}
	if err := p.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	acc := &testutil.Accumulator{}
	_ = p.Gather(acc)

	if len(acc.Errors) != 0 {
		t.Fatalf("expected 0 errors, got %d", len(acc.Errors))
	}
	if len(acc.Metrics) != 0 {
		t.Fatalf("expected 0 metrics for empty JSON, got %d", len(acc.Metrics))
	}
}

func TestGather_EmptyJSONArray(t *testing.T) {
	mock := NewMockRunner()
	mock.SetOutput("sudo nicctl show port statistics --json", []byte(`[]`))

	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	p := &NicctlPlugin{
		CommandGroups: []*CommandGroup{
			{Interval: "5s", Commands: []string{"nicctl show port statistics"}},
		},
		runner:  mock,
		nowFunc: func() time.Time { return now },
	}
	if err := p.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	acc := &testutil.Accumulator{}
	_ = p.Gather(acc)

	if len(acc.Errors) != 0 {
		t.Fatalf("expected 0 errors, got %d", len(acc.Errors))
	}
	if len(acc.Metrics) != 0 {
		t.Fatalf("expected 0 metrics for empty array, got %d", len(acc.Metrics))
	}
}

func TestGather_TruncatedJSON(t *testing.T) {
	mock := NewMockRunner()
	mock.SetOutput("sudo nicctl show port statistics --json", []byte(`{"tx": 100, "rx"`))

	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	p := &NicctlPlugin{
		CommandGroups: []*CommandGroup{
			{Interval: "5s", Commands: []string{"nicctl show port statistics"}},
		},
		runner:  mock,
		nowFunc: func() time.Time { return now },
	}
	if err := p.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	acc := &testutil.Accumulator{}
	_ = p.Gather(acc)

	if len(acc.Errors) != 1 {
		t.Fatalf("expected 1 error for truncated JSON, got %d", len(acc.Errors))
	}
	if len(acc.Metrics) != 0 {
		t.Fatalf("expected 0 metrics, got %d", len(acc.Metrics))
	}
}

func TestGather_EmptyOutput(t *testing.T) {
	mock := NewMockRunner()
	mock.SetOutput("sudo nicctl show port statistics --json", []byte(``))

	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	p := &NicctlPlugin{
		CommandGroups: []*CommandGroup{
			{Interval: "5s", Commands: []string{"nicctl show port statistics"}},
		},
		runner:  mock,
		nowFunc: func() time.Time { return now },
	}
	if err := p.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	acc := &testutil.Accumulator{}
	_ = p.Gather(acc)

	if len(acc.Errors) != 1 {
		t.Fatalf("expected 1 error for empty output, got %d", len(acc.Errors))
	}
	if len(acc.Metrics) != 0 {
		t.Fatalf("expected 0 metrics, got %d", len(acc.Metrics))
	}
}

func TestGather_JSONAllNulls(t *testing.T) {
	mock := NewMockRunner()
	mock.SetOutput("sudo nicctl show port statistics --json", []byte(`{"a": null, "b": null}`))

	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	p := &NicctlPlugin{
		CommandGroups: []*CommandGroup{
			{Interval: "5s", Commands: []string{"nicctl show port statistics"}},
		},
		runner:  mock,
		nowFunc: func() time.Time { return now },
	}
	if err := p.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	acc := &testutil.Accumulator{}
	_ = p.Gather(acc)

	if len(acc.Errors) != 0 {
		t.Fatalf("expected 0 errors, got %d", len(acc.Errors))
	}
	if len(acc.Metrics) != 0 {
		t.Fatalf("expected 0 metrics when all values are null, got %d", len(acc.Metrics))
	}
}

func TestGather_CommandFailureDoesNotUpdateLastRun(t *testing.T) {
	mock := NewMockRunner()
	mock.SetError("sudo nicctl show port statistics --json", fmt.Errorf("command failed"))

	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	p := &NicctlPlugin{
		CommandGroups: []*CommandGroup{
			{Interval: "5s", Commands: []string{"nicctl show port statistics"}},
		},
		runner:  mock,
		nowFunc: func() time.Time { return now },
	}
	if err := p.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	acc := &testutil.Accumulator{}
	_ = p.Gather(acc)
	if mock.CallCount() != 1 {
		t.Fatalf("expected 1 call, got %d", mock.CallCount())
	}

	// Should retry immediately on next gather since lastRun was not set
	now = now.Add(1 * time.Second)
	mock.SetOutput("sudo nicctl show port statistics --json", []byte(`{"tx": 1}`))
	delete(mock.errors, "sudo nicctl show port statistics --json")

	acc = &testutil.Accumulator{}
	_ = p.Gather(acc)
	if mock.CallCount() != 2 {
		t.Fatalf("expected 2 calls (retry after failure), got %d", mock.CallCount())
	}
	if len(acc.Metrics) != 1 {
		t.Fatalf("expected 1 metric on retry, got %d", len(acc.Metrics))
	}
}

// Ensure NicctlPlugin satisfies telegraf.Input.
var _ telegraf.Input = (*NicctlPlugin)(nil)
