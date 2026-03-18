package gen

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tailscale/hujson"
)

// jsondef is a stateful wrapper around a parsed hujson value whose
// top level is either an array or an object. Use one of the constructors below
// to load from disk.
type jsondef struct {
	ast hujson.Value
}

// loadJsonFile reads path as a hujson value (JSONC: comments and trailing
// commas are preserved). It accepts both top-level arrays and top-level
// objects. It returns an error when the file does not exist; use
// [loadJsonArrayFile] or [loadJsonObjectFile] for callers that want a default.
func loadJsonFile(path string) (*jsondef, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	val, err := hujson.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("json parse %q: %w", path, err)
	}

	switch val.Value.(type) {
	case *hujson.Array, *hujson.Object:
		// ok
	default:
		return nil, fmt.Errorf("json parse %q: expected a json array or object at top level", path)
	}

	return &jsondef{ast: val}, nil
}

// loadJsonArrayFile is like [loadJsonFile] but returns an empty-array
// definition when the file does not exist, so callers can unconditionally
// append to it.
func loadJsonArrayFile(path string) (*jsondef, error) {
	def, err := loadJsonFile(path)
	if errors.Is(err, os.ErrNotExist) {
		val, _ := hujson.Parse([]byte("[]\n"))
		return &jsondef{ast: val}, nil
	}
	return def, err
}

// loadJsonObjectFile is like [loadJsonFile] but returns an empty-object
// definition when the file does not exist, so callers can unconditionally
// set fields on it.
func loadJsonObjectFile(path string) (*jsondef, error) {
	def, err := loadJsonFile(path)
	if errors.Is(err, os.ErrNotExist) {
		val, _ := hujson.Parse([]byte("{}\n"))
		return &jsondef{ast: val}, nil
	}
	return def, err
}

// getRootArray returns the top-level hujson.Array, or nil if the root is not an array.
func (def *jsondef) getRootArray() *hujson.Array {
	return def.elemAsArray(def.ast)
}

// getRootObject returns the top-level hujson.Object, or nil if the root is not an object.
func (def *jsondef) getRootObject() *hujson.Object {
	return def.elemAsObject(def.ast)
}

// elemAsArray returns the array value of elem, or nil if elem is not an array.
func (def *jsondef) elemAsArray(elem hujson.Value) *hujson.Array {
	arr, _ := elem.Value.(*hujson.Array)
	return arr
}

// elemAsObject returns the object value of elem, or nil if elem is not an object.
func (def *jsondef) elemAsObject(elem hujson.Value) *hujson.Object {
	obj, _ := elem.Value.(*hujson.Object)
	return obj
}

// save writes the definition to disk byte-for-byte (no reformatting).
// Parent directories are created as needed.
func (def *jsondef) save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	def.ast.Format()
	data := def.ast.Pack()
	if len(data) == 0 || data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}

	return os.WriteFile(path, data, 0o644)
}

// getMemberKey returns the unquoted string key of an ObjectMember.
func (def *jsondef) getMemberKey(member hujson.ObjectMember) string {
	if lit, ok := member.Name.Value.(hujson.Literal); ok {
		return lit.String()
	}
	return ""
}

// getMemberValue returns the unquoted string value of an ObjectMember.
func (def *jsondef) getMemberValue(member hujson.ObjectMember) string {
	if lit, ok := member.Value.Value.(hujson.Literal); ok {
		return lit.String()
	}
	return ""
}

// findObjectMember returns the index of the member with the given key, or -1
// if obj is nil or the key is absent.
func (def *jsondef) findObjectMember(obj *hujson.Object, key string) int {
	if obj == nil {
		return -1
	}

	for idx, member := range obj.Members {
		if def.getMemberKey(member) == key {
			return idx
		}
	}

	return -1
}

// readObjectMemberValue returns the unquoted string value of the member with the given key,
// or an empty string if the key is not found or the value is not a literal.
func (def *jsondef) readObjectMemberValue(obj *hujson.Object, key string) string {
	if idx := def.findObjectMember(obj, key); idx >= 0 {
		return def.getMemberValue(obj.Members[idx])
	}

	return ""
}

// setObjectMember sets the value of an existing member with the given key, or
// creates a new member if the key doesn't exist. The value is marshalled
// through JSON into the hujson AST, preserving any comments or whitespace
// on the existing member.
func (def *jsondef) setObjectMember(obj *hujson.Object, key string, value any) error {
	serialized, err := json.Marshal(value)
	if err != nil {
		return err
	}

	val, err := hujson.Parse(serialized)
	if err != nil {
		return err
	}

	if idx := def.findObjectMember(obj, key); idx >= 0 {
		obj.Members[idx].Value.Value = val.Value
		return nil
	}

	obj.Members = append(
		obj.Members,
		hujson.ObjectMember{
			Name:  hujson.Value{Value: hujson.String(key)},
			Value: val,
		},
	)

	return nil
}

// deleteObjectMember removes the member with the given key. No-op if absent.
func (def *jsondef) deleteObjectMember(obj *hujson.Object, key string) {
	out := obj.Members[:0]
	for _, member := range obj.Members {
		if def.getMemberKey(member) != key {
			out = append(out, member)
		}
	}
	obj.Members = out
}

// appendArrayElement marshals value as JSON, parses it as hujson, and appends it
// to the array.
func (def *jsondef) appendArrayElement(arr *hujson.Array, value any) error {
	serialized, err := json.Marshal(value)
	if err != nil {
		return err
	}

	val, err := hujson.Parse(serialized)
	if err != nil {
		return err
	}

	arr.Elements = append(arr.Elements, val)

	return nil
}

// findArrayElementByLabel returns the 0-based array index of the first element whose
// "label" member equals label, or -1 if not found.
func (def *jsondef) findArrayElementByLabel(arr *hujson.Array, label string) int {
	for idx, elem := range arr.Elements {
		if obj := def.elemAsObject(elem); obj != nil {
			if def.readObjectMemberValue(obj, "label") == label {
				return idx
			}
		}
	}
	return -1
}
