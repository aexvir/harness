package gen

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tailscale/hujson"
)

// jsonDefinition is a stateful wrapper around a parsed hujson value whose
// top level is either an array or an object. Use one of the constructors below
// to load from disk.
type jsonDefinition struct {
	ast hujson.Value
}

// loadJsonFile reads path as a hujson value (JSONC: comments and trailing
// commas are preserved). It accepts both top-level arrays and top-level
// objects. It returns an error when the file does not exist; use
// [loadJsonArrayFile] or [loadJsonObjectFile] for callers that want a default.
func loadJsonFile(path string) (*jsonDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	v, err := hujson.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("json parse %q: %w", path, err)
	}
	switch v.Value.(type) {
	case *hujson.Array, *hujson.Object:
		// ok
	default:
		return nil, fmt.Errorf("json parse %q: expected a json array or object at top level", path)
	}
	return &jsonDefinition{ast: v}, nil
}

// loadJsonArrayFile is like [loadJsonFile] but returns an empty-array
// definition when the file does not exist, so callers can unconditionally
// append to it.
func loadJsonArrayFile(path string) (*jsonDefinition, error) {
	def, err := loadJsonFile(path)
	if errors.Is(err, os.ErrNotExist) {
		v, _ := hujson.Parse([]byte("[]\n"))
		return &jsonDefinition{ast: v}, nil
	}
	return def, err
}

// loadJsonObjectFile is like [loadJsonFile] but returns an empty-object
// definition when the file does not exist, so callers can unconditionally
// set fields on it.
func loadJsonObjectFile(path string) (*jsonDefinition, error) {
	def, err := loadJsonFile(path)
	if errors.Is(err, os.ErrNotExist) {
		v, _ := hujson.Parse([]byte("{}\n"))
		return &jsonDefinition{ast: v}, nil
	}
	return def, err
}

// Array returns the top-level hujson.Array, or nil if the root is not an array.
func (d *jsonDefinition) Array() *hujson.Array {
	arr, _ := d.ast.Value.(*hujson.Array)
	return arr
}

// Object returns the top-level hujson.Object, or nil if the root is not an object.
func (d *jsonDefinition) Object() *hujson.Object {
	obj, _ := d.ast.Value.(*hujson.Object)
	return obj
}

// WriteToFile writes the definition to disk byte-for-byte (no reformatting).
// Parent directories are created as needed.
func (d *jsonDefinition) WriteToFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	d.ast.Format()
	data := d.ast.Pack()
	if len(data) == 0 || data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}
	return os.WriteFile(path, data, 0o644)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// memberKey returns the unquoted string key of an ObjectMember.
func (d *jsonDefinition) memberKey(m hujson.ObjectMember) string {
	if lit, ok := m.Name.Value.(hujson.Literal); ok {
		return lit.String()
	}
	return ""
}

// findMember returns the index of the member with the given key, or -1.
func (d *jsonDefinition) findMember(obj *hujson.Object, key string) int {
	for i := range obj.Members {
		if d.memberKey(obj.Members[i]) == key {
			return i
		}
	}
	return -1
}

// setMember sets the value of an existing member with the given key, or
// creates a new member if the key doesn't exist. The value is marshalled
// through JSON into the hujson AST, preserving any comments or whitespace
// on the existing member.
func (d *jsonDefinition) setMember(obj *hujson.Object, key string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	val, err := hujson.Parse(b)
	if err != nil {
		return err
	}
	if i := d.findMember(obj, key); i >= 0 {
		obj.Members[i].Value.Value = val.Value
		return nil
	}
	obj.Members = append(obj.Members, hujson.ObjectMember{
		Name:  hujson.Value{Value: hujson.String(key)},
		Value: val,
	})
	return nil
}

// deleteMember removes the member with the given key. No-op if absent.
func (d *jsonDefinition) deleteMember(obj *hujson.Object, key string) {
	out := obj.Members[:0]
	for _, m := range obj.Members {
		if d.memberKey(m) != key {
			out = append(out, m)
		}
	}
	obj.Members = out
}

// appendElement marshals v as JSON, parses it as hujson, and appends it
// to the array. Each new element is preceded by a newline. If the value
// is an object, a newline is injected before the first member so that
// Format() expands it into multi-line form.
func (d *jsonDefinition) appendElement(arr *hujson.Array, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	val, err := hujson.Parse(b)
	if err != nil {
		return err
	}
	if obj, ok := val.Value.(*hujson.Object); ok && len(obj.Members) > 0 {
		obj.Members[0].Name.BeforeExtra = hujson.Extra("\n")
	}
	val.BeforeExtra = hujson.Extra("\n")
	arr.Elements = append(arr.Elements, val)
	return nil
}
