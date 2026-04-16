package nicctl

import (
	"testing"
)

func TestFlattenJSON_FlatObject(t *testing.T) {
	input := []byte(`{"packets_tx": 100, "packets_rx": 200}`)
	result, err := FlattenJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertInt64(t, result, "packets_tx", 100)
	assertInt64(t, result, "packets_rx", 200)
}

func TestFlattenJSON_NestedObject(t *testing.T) {
	input := []byte(`{"port": {"stats": {"tx": 10, "rx": 20}}}`)
	result, err := FlattenJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertInt64(t, result, "port_stats_tx", 10)
	assertInt64(t, result, "port_stats_rx", 20)
}

func TestFlattenJSON_Array(t *testing.T) {
	input := []byte(`{"ports": [1, 2, 3]}`)
	result, err := FlattenJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertInt64(t, result, "ports_0", 1)
	assertInt64(t, result, "ports_1", 2)
	assertInt64(t, result, "ports_2", 3)
}

func TestFlattenJSON_Nulls(t *testing.T) {
	input := []byte(`{"a": 1, "b": null, "c": 3}`)
	result, err := FlattenJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result["b"]; ok {
		t.Error("null values should be skipped")
	}
	assertInt64(t, result, "a", 1)
	assertInt64(t, result, "c", 3)
}

func TestFlattenJSON_Booleans(t *testing.T) {
	input := []byte(`{"enabled": true, "disabled": false}`)
	result, err := FlattenJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["enabled"] != true {
		t.Errorf("expected true, got %v", result["enabled"])
	}
	if result["disabled"] != false {
		t.Errorf("expected false, got %v", result["disabled"])
	}
}

func TestFlattenJSON_IntNarrowing(t *testing.T) {
	input := []byte(`{"count": 42, "rate": 3.14}`)
	result, err := FlattenJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v, ok := result["count"].(int64); !ok || v != 42 {
		t.Errorf("expected int64(42), got %T(%v)", result["count"], result["count"])
	}
	if v, ok := result["rate"].(float64); !ok || v != 3.14 {
		t.Errorf("expected float64(3.14), got %T(%v)", result["rate"], result["rate"])
	}
}

func TestFlattenJSON_InvalidJSON(t *testing.T) {
	input := []byte(`not json`)
	_, err := FlattenJSON(input)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestFlattenJSON_KeySanitization(t *testing.T) {
	input := []byte(`{"some key": 1, "a.b": 2, "c,d": 3, "e=f": 4}`)
	result, err := FlattenJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertInt64(t, result, "some_key", 1)
	assertInt64(t, result, "a_b", 2)
	assertInt64(t, result, "c_d", 3)
	assertInt64(t, result, "e_f", 4)
}

func TestFlattenJSON_DeepNesting(t *testing.T) {
	input := []byte(`{"a": {"b": {"c": {"d": 42}}}}`)
	result, err := FlattenJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertInt64(t, result, "a_b_c_d", 42)
}

func TestFlattenJSON_Strings(t *testing.T) {
	input := []byte(`{"name": "eth0", "status": "up"}`)
	result, err := FlattenJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["name"] != "eth0" {
		t.Errorf("expected eth0, got %v", result["name"])
	}
	if result["status"] != "up" {
		t.Errorf("expected up, got %v", result["status"])
	}
}

func TestFlattenJSON_NestedArray(t *testing.T) {
	input := []byte(`{"items": [{"name": "a"}, {"name": "b"}]}`)
	result, err := FlattenJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["items_0_name"] != "a" {
		t.Errorf("expected a, got %v", result["items_0_name"])
	}
	if result["items_1_name"] != "b" {
		t.Errorf("expected b, got %v", result["items_1_name"])
	}
}

func TestFlattenJSON_EmptyObject(t *testing.T) {
	input := []byte(`{}`)
	result, err := FlattenJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func assertInt64(t *testing.T, result map[string]interface{}, key string, expected int64) {
	t.Helper()
	v, ok := result[key]
	if !ok {
		t.Errorf("key %q not found", key)
		return
	}
	iv, ok := v.(int64)
	if !ok {
		t.Errorf("key %q: expected int64, got %T(%v)", key, v, v)
		return
	}
	if iv != expected {
		t.Errorf("key %q: expected %d, got %d", key, expected, iv)
	}
}
