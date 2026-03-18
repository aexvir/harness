package gen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tailscale/hujson"
)

const (
	testdataJSONCArray = "testdata/json_array.json"
	testdataJSONObject = "testdata/json_object.json"
)

func TestLoadJsonFile(t *testing.T) {
	t.Run(
		"missing file returns error",
		func(t *testing.T) {
			_, err := loadJsonFile(filepath.Join(t.TempDir(), "nonexistent.json"))
			require.Error(t, err)
		},
	)

	t.Run(
		"JSONC array with comments and trailing commas is parsed",
		func(t *testing.T) {
			def, err := loadJsonFile(testdataJSONCArray)
			require.NoError(t, err)
			arr := def.getRootArray()
			require.NotNil(t, arr)
			assert.Len(t, arr.Elements, 1)
		},
	)

	t.Run(
		"JSONC object with comments and trailing commas is parsed",
		func(t *testing.T) {
			def, err := loadJsonFile(testdataJSONObject)
			require.NoError(t, err)
			obj := def.getRootObject()
			require.NotNil(t, obj)
			memberIdx := def.findObjectMember(obj, "key")
			require.GreaterOrEqual(t, memberIdx, 0)
			assert.Equal(t, "val", obj.Members[memberIdx].Value.Value.(hujson.Literal).String())
		},
	)
}

func TestLoadJsonArrayFile(t *testing.T) {
	t.Run(
		"missing file returns empty array definition",
		func(t *testing.T) {
			def, err := loadJsonArrayFile(filepath.Join(t.TempDir(), "nonexistent.json"))
			require.NoError(t, err)
			arr := def.getRootArray()
			require.NotNil(t, arr)
			assert.Empty(t, arr.Elements)
		},
	)

	t.Run(
		"existing array file is loaded normally",
		func(t *testing.T) {
			def, err := loadJsonArrayFile(testdataJSONCArray)
			require.NoError(t, err)
			arr := def.getRootArray()
			require.NotNil(t, arr)
			assert.Len(t, arr.Elements, 1)
		},
	)
}

func TestLoadJsonObjectFile(t *testing.T) {
	t.Run(
		"missing file returns empty object definition",
		func(t *testing.T) {
			def, err := loadJsonObjectFile(filepath.Join(t.TempDir(), "nonexistent.json"))
			require.NoError(t, err)
			obj := def.getRootObject()
			require.NotNil(t, obj)
			assert.Empty(t, obj.Members)
		},
	)

	t.Run(
		"existing object file is loaded normally",
		func(t *testing.T) {
			def, err := loadJsonObjectFile(testdataJSONObject)
			require.NoError(t, err)
			obj := def.getRootObject()
			require.NotNil(t, obj)
			memberIdx := def.findObjectMember(obj, "key")
			require.GreaterOrEqual(t, memberIdx, 0)
			assert.Equal(t, "val", obj.Members[memberIdx].Value.Value.(hujson.Literal).String())
		},
	)
}

func TestFindMember(t *testing.T) {
	v, _ := hujson.Parse([]byte(`{"a":1,"b":2,"c":3}`))
	def := jsonDefFrom(v)
	obj := v.Value.(*hujson.Object)

	t.Run(
		"finds existing key",
		func(t *testing.T) {
			assert.Equal(t, 0, def.findObjectMember(obj, "a"))
			assert.Equal(t, 1, def.findObjectMember(obj, "b"))
			assert.Equal(t, 2, def.findObjectMember(obj, "c"))
		},
	)

	t.Run(
		"returns -1 for missing key",
		func(t *testing.T) {
			assert.Equal(t, -1, def.findObjectMember(obj, "missing"))
		},
	)
}

func TestSetMember(t *testing.T) {
	t.Run(
		"updates existing key",
		func(t *testing.T) {
			v, _ := hujson.Parse([]byte(`{"x":"old"}`))
			def := jsonDefFrom(v)
			obj := v.Value.(*hujson.Object)
			require.NoError(t, def.setObjectMember(obj, "x", "new"))
			assert.Equal(t, `{"x":"new"}`, string(v.Pack()))
		},
	)

	t.Run(
		"creates missing key",
		func(t *testing.T) {
			v, _ := hujson.Parse([]byte(`{}`))
			def := jsonDefFrom(v)
			obj := v.Value.(*hujson.Object)
			require.NoError(t, def.setObjectMember(obj, "foo", "bar"))
			assert.Equal(t, `{"foo":"bar"}`, string(v.Pack()))
		},
	)

	t.Run(
		"preserves other members",
		func(t *testing.T) {
			v, _ := hujson.Parse([]byte(`{"a":1,"b":2}`))
			def := jsonDefFrom(v)
			obj := v.Value.(*hujson.Object)
			require.NoError(t, def.setObjectMember(obj, "a", 99))
			assert.Equal(t, `{"a":99,"b":2}`, string(v.Pack()))
		},
	)

	t.Run(
		"replaces with complex value",
		func(t *testing.T) {
			v, _ := hujson.Parse([]byte(`{"x":null}`))
			def := jsonDefFrom(v)
			obj := v.Value.(*hujson.Object)
			require.NoError(t, def.setObjectMember(obj, "x", []string{"a", "b"}))
			assert.Equal(t, `{"x":["a","b"]}`, string(v.Pack()))
		},
	)
}

func TestDeleteMember(t *testing.T) {
	t.Run(
		"removes existing key",
		func(t *testing.T) {
			v, _ := hujson.Parse([]byte(`{"a":"1","b":"2"}`))
			def := jsonDefFrom(v)
			obj := v.Value.(*hujson.Object)
			def.deleteObjectMember(obj, "a")
			assert.Equal(t, `{"b":"2"}`, string(v.Pack()))
		},
	)

	t.Run(
		"no-op when key absent",
		func(t *testing.T) {
			v, _ := hujson.Parse([]byte(`{"a":"1"}`))
			def := jsonDefFrom(v)
			obj := v.Value.(*hujson.Object)
			def.deleteObjectMember(obj, "missing")
			assert.Equal(t, `{"a":"1"}`, string(v.Pack()))
		},
	)
}

func TestAppendElement(t *testing.T) {
	t.Run(
		"appends to array",
		func(t *testing.T) {
			v, _ := hujson.Parse([]byte(`[{"x":1}]`))
			def := jsonDefFrom(v)
			arr := v.Value.(*hujson.Array)
			require.NoError(t, def.appendArrayElement(arr, map[string]int{"x": 2}))
			assert.Len(t, arr.Elements, 2)
			assert.Equal(t, `[{"x":1},{"x":2}]`, string(v.Pack()))
		},
	)

	t.Run(
		"appends string value",
		func(t *testing.T) {
			v, _ := hujson.Parse([]byte(`["a"]`))
			def := jsonDefFrom(v)
			arr := v.Value.(*hujson.Array)
			require.NoError(t, def.appendArrayElement(arr, "b"))
			assert.Len(t, arr.Elements, 2)
			assert.Equal(t, `["a","b"]`, string(v.Pack()))
		},
	)
}

func TestSave(t *testing.T) {
	t.Run(
		"creates parent dirs and ends with newline",
		func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "a", "b", "f.json")
			def := &jsondef{}
			def.ast, _ = hujson.Parse([]byte(`[]`))
			require.NoError(t, def.save(path))

			data, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, byte('\n'), data[len(data)-1])
		},
	)

	t.Run(
		"array preserves comments and formatting is idempotent",
		func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "f.json")
			src, err := os.ReadFile(testdataJSONCArray)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(path, src, 0o644))

			def, err := loadJsonFile(path)
			require.NoError(t, err)
			require.NoError(t, def.save(path))

			first, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Contains(t, string(first), "// comment", "comments must survive")

			// Write again: output must be stable.
			def2, err := loadJsonFile(path)
			require.NoError(t, err)
			require.NoError(t, def2.save(path))
			second, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, first, second, "formatting must be idempotent")
		},
	)

	t.Run(
		"object preserves comments and formatting is idempotent",
		func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "f.json")
			src, err := os.ReadFile(testdataJSONObject)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(path, src, 0o644))

			def, err := loadJsonFile(path)
			require.NoError(t, err)
			require.NoError(t, def.save(path))

			first, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Contains(t, string(first), "// JSONC comment", "comments must survive")

			// Write again: output must be stable.
			def2, err := loadJsonFile(path)
			require.NoError(t, err)
			require.NoError(t, def2.save(path))
			second, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, first, second, "formatting must be idempotent")
		},
	)
}

func TestMemberKey(t *testing.T) {
	val, _ := hujson.Parse([]byte(`{"hello":"world"}`))
	def := jsonDefFrom(val)
	obj := val.Value.(*hujson.Object)
	assert.Equal(t, "hello", def.getMemberKey(obj.Members[0]))
}

func TestTasksJsonRoundtrip(t *testing.T) {
	src, err := os.ReadFile("testdata/tasks.json")
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "tasks.json")
	require.NoError(t, os.WriteFile(path, src, 0o644))

	def, err := loadJsonFile(path)
	require.NoError(t, err)

	arr := def.getRootArray()
	require.NotNil(t, arr)

	// Find "mage: fmt" by label and patch its command.
	idx := def.findArrayElementByLabel(arr, "mage: fmt")
	require.GreaterOrEqual(t, idx, 0)
	fmtObj := arr.Elements[idx].Value.(*hujson.Object)
	require.NoError(t, def.setObjectMember(fmtObj, "command", "mage2"))

	// Delete args from "mage: test".
	idx = def.findArrayElementByLabel(arr, "mage: test")
	require.GreaterOrEqual(t, idx, 0)
	testObj := arr.Elements[idx].Value.(*hujson.Object)
	def.deleteObjectMember(testObj, "args")

	// Append a new entry.
	require.NoError(t, def.appendArrayElement(arr, map[string]string{"label": "new", "command": "true"}))

	// Remove the "custom" entry by filtering.
	kept := arr.Elements[:0]
	for _, elem := range arr.Elements {
		if def.readObjectMemberValue(def.elemAsObject(elem), "label") != "custom" {
			kept = append(kept, elem)
		}
	}
	arr.Elements = kept

	require.NoError(t, def.save(path))

	got, err := os.ReadFile(path)
	require.NoError(t, err)

	packed := string(got)
	assert.Contains(t, packed, `"mage2"`)
	assert.NotContains(t, packed, `"custom"`)
	assert.Contains(t, packed, `"new"`)
	// mage: test still present but no longer has args
	assert.Contains(t, packed, `"mage: test"`)
	// mage: fmt still has its args
	assert.Contains(t, packed, `"mage: fmt"`)
	assert.Contains(t, packed, `"args"`)
}

// jsonDefFrom wraps a raw hujson.Value in a jsonDefinition so the helper methods
// are callable in unit tests that don't go through loadJsonFile.
func jsonDefFrom(v hujson.Value) *jsondef {
	return &jsondef{ast: v}
}
