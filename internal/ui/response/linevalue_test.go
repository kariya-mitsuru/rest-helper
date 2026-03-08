// SPDX-License-Identifier: MIT

package response

import (
	"testing"
)

func TestBuildJSONLineInfos_NestedObject(t *testing.T) {
	body := `{"a":{"b":1},"c":[1,2]}`
	infos := buildJSONLineInfos(body)

	// Pretty-printed:
	// 0: {
	// 1:   "a": {
	// 2:     "b": 1
	// 3:   },
	// 4:   "c": [
	// 5:     1,
	// 6:     2
	// 7:   ]
	// 8: }

	tests := []struct {
		line    int
		key     string
		value   string
		raw     string
		isEmpty bool
	}{
		{0, "", "", "", true},                 // {
		{1, "a", `{"b":1}`, `{"b":1}`, false}, // "a": {
		{2, "b", "1", "1", false},             // "b": 1
		{3, "", "", "", true},                 // },
		{4, "c", `[1,2]`, `[1,2]`, false},     // "c": [
		{5, "", "1", "1", false},              // 1,
		{6, "", "2", "2", false},              // 2
		{7, "", "", "", true},                 // ]
		{8, "", "", "", true},                 // }
	}

	if len(infos) != 9 {
		t.Fatalf("expected 9 lines, got %d", len(infos))
	}

	for _, tt := range tests {
		info := infos[tt.line]
		if tt.isEmpty {
			if info.key != "" || info.value != "" {
				t.Errorf("line %d: expected empty, got key=%q value=%q", tt.line, info.key, info.value)
			}
			continue
		}
		if info.key != tt.key {
			t.Errorf("line %d: key: got %q, want %q", tt.line, info.key, tt.key)
		}
		if info.value != tt.value {
			t.Errorf("line %d: value: got %q, want %q", tt.line, info.value, tt.value)
		}
		if info.raw != tt.raw {
			t.Errorf("line %d: raw: got %q, want %q", tt.line, info.raw, tt.raw)
		}
	}
}

func TestBuildJSONLineInfos_ArrayOfObjects(t *testing.T) {
	body := `[{"a":1},{"b":2}]`
	infos := buildJSONLineInfos(body)

	// Pretty-printed:
	// 0: [
	// 1:   {
	// 2:     "a": 1
	// 3:   },
	// 4:   {
	// 5:     "b": 2
	// 6:   }
	// 7: ]

	if len(infos) != 8 {
		t.Fatalf("expected 8 lines, got %d", len(infos))
	}

	// Line 1: bare { array element → should have subtree value
	if infos[1].value != `{"a":1}` {
		t.Errorf("line 1: value: got %q, want %q", infos[1].value, `{"a":1}`)
	}
	// Line 4: bare { array element
	if infos[4].value != `{"b":2}` {
		t.Errorf("line 4: value: got %q, want %q", infos[4].value, `{"b":2}`)
	}
	// Line 2: leaf
	if infos[2].key != "a" || infos[2].value != "1" {
		t.Errorf("line 2: got key=%q value=%q, want key=a value=1", infos[2].key, infos[2].value)
	}
}

func TestBuildJSONLineInfos_EmptyContainers(t *testing.T) {
	body := `{"a":{},"b":[]}`
	infos := buildJSONLineInfos(body)

	// Pretty-printed:
	// 0: {
	// 1:   "a": {},
	// 2:   "b": []
	// 3: }

	if len(infos) != 4 {
		t.Fatalf("expected 4 lines, got %d", len(infos))
	}

	if infos[1].key != "a" || infos[1].value != "{}" {
		t.Errorf("line 1: got key=%q value=%q, want key=a value={}", infos[1].key, infos[1].value)
	}
	if infos[2].key != "b" || infos[2].value != "[]" {
		t.Errorf("line 2: got key=%q value=%q, want key=b value=[]", infos[2].key, infos[2].value)
	}
}

func TestBuildYAMLLineInfos_Nested(t *testing.T) {
	body := `{"a":{"b":1},"c":[1,2]}`
	infos := buildYAMLLineInfos(body)

	// YAML output:
	// 0: a:
	// 1:   b: 1
	// 2: c:
	// 3:   - 1
	// 4:   - 2

	if len(infos) != 5 {
		t.Fatalf("expected 5 lines, got %d", len(infos))
	}

	// Line 0: key "a" with nested value
	if infos[0].key != "a" {
		t.Errorf("line 0: key: got %q, want %q", infos[0].key, "a")
	}
	if infos[0].value != `{"b":1}` {
		t.Errorf("line 0: value: got %q, want %q", infos[0].value, `{"b":1}`)
	}

	// Line 1: leaf
	if infos[1].key != "b" || infos[1].value != "1" {
		t.Errorf("line 1: got key=%q value=%q, want key=b value=1", infos[1].key, infos[1].value)
	}

	// Line 2: key "c" with nested array
	if infos[2].key != "c" {
		t.Errorf("line 2: key: got %q, want %q", infos[2].key, "c")
	}
	if infos[2].value != `[1,2]` {
		t.Errorf("line 2: value: got %q, want %q", infos[2].value, `[1,2]`)
	}
}

func TestFormatJSON(t *testing.T) {
	info := lineInfo{key: "name", value: "hello", raw: `"hello"`}
	got := info.formatJSON()
	want := `"name": "hello"`
	if got != want {
		t.Errorf("formatJSON: got %q, want %q", got, want)
	}

	info2 := lineInfo{key: "items", value: `[1,2]`, raw: `[1,2]`}
	got2 := info2.formatJSON()
	want2 := `"items": [1,2]`
	if got2 != want2 {
		t.Errorf("formatJSON: got %q, want %q", got2, want2)
	}
}

func TestFormatYAML_Scalar(t *testing.T) {
	info := lineInfo{key: "name", value: "hello", raw: `"hello"`}
	got := info.formatYAML()
	want := "name: hello"
	if got != want {
		t.Errorf("formatYAML scalar: got %q, want %q", got, want)
	}
}

func TestFormatYAML_Structured(t *testing.T) {
	info := lineInfo{key: "items", value: `[1,2]`, raw: `[1,2]`}
	got := info.formatYAML()
	want := "items:\n    - 1\n    - 2"
	if got != want {
		t.Errorf("formatYAML structured: got %q, want %q", got, want)
	}
}
