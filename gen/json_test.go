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
	testdataJSONCArray = "testdata/jsonc_array.json"
	testdataJSONObject = "testdata/json_object.json"
	testdataTasks      = "testdata/tasks.json"
)

// defFrom wraps a raw hujson.Value in a jsonDefinition so the helper methods
// are callable in unit tests that don't go through loadJsonFile.
func defFrom(v hujson.Value) *jsonDefinition {
	return &jsonDefinition{ast: v}
}

func TestLoadJsonFile(t *testing.T) {
	t.Run("missing file returns error", func(t *testing.T) {
		_, err := loadJsonFile(filepath.Join(t.TempDir(), "nonexistent.json"))
		require.Error(t, err)
	})

	t.Run("JSONC array with comments and trailing commas is parsed", func(t *testing.T) {
		def, err := loadJsonFile(testdataJSONCArray)
		require.NoError(t, err)
		arr := def.Array()
		require.NotNil(t, arr)
		assert.Len(t, arr.Elements, 1)
	})

	t.Run("JSONC object with comments and trailing commas is parsed", func(t *testing.T) {
		def, err := loadJsonFile(testdataJSONObject)
		require.NoError(t, err)
		assert.NotNil(t, def.Object())
	})
}

func TestLoadJsonArrayFile(t *testing.T) {
	t.Run("missing file returns empty array definition", func(t *testing.T) {
		def, err := loadJsonArrayFile(filepath.Join(t.TempDir(), "nonexistent.json"))
		require.NoError(t, err)
		arr := def.Array()
		require.NotNil(t, arr)
		assert.Empty(t, arr.Elements)
	})

	t.Run("existing array file is loaded normally", func(t *testing.T) {
		def, err := loadJsonArrayFile(testdataJSONCArray)
		require.NoError(t, err)
		arr := def.Array()
		require.NotNil(t, arr)
		assert.Len(t, arr.Elements, 1)
	})
}

func TestLoadJsonObjectFile(t *testing.T) {
	t.Run("missing file returns empty object definition", func(t *testing.T) {
		def, err := loadJsonObjectFile(filepath.Join(t.TempDir(), "nonexistent.json"))
		require.NoError(t, err)
		obj := def.Object()
		require.NotNil(t, obj)
		assert.Empty(t, obj.Members)
	})

	t.Run("existing object file is loaded normally", func(t *testing.T) {
		def, err := loadJsonObjectFile(testdataJSONObject)
		require.NoError(t, err)
		assert.NotNil(t, def.Object())
	})
}

func TestFindMember(t *testing.T) {
	v, _ := hujson.Parse([]byte(`{"a":1,"b":2,"c":3}`))
	def := defFrom(v)
	obj := v.Value.(*hujson.Object)

	t.Run("finds existing key", func(t *testing.T) {
		assert.Equal(t, 0, def.findMember(obj, "a"))
		assert.Equal(t, 1, def.findMember(obj, "b"))
		assert.Equal(t, 2, def.findMember(obj, "c"))
	})

	t.Run("returns -1 for missing key", func(t *testing.T) {
		assert.Equal(t, -1, def.findMember(obj, "missing"))
	})
}

func TestSetMember(t *testing.T) {
	t.Run("updates existing key", func(t *testing.T) {
		v, _ := hujson.Parse([]byte(`{"x":"old"}`))
		def := defFrom(v)
		obj := v.Value.(*hujson.Object)
		require.NoError(t, def.setMember(obj, "x", "new"))
		assert.Equal(t, `{"x":"new"}`, string(v.Pack()))
	})

	t.Run("creates missing key", func(t *testing.T) {
		v, _ := hujson.Parse([]byte(`{}`))
		def := defFrom(v)
		obj := v.Value.(*hujson.Object)
		require.NoError(t, def.setMember(obj, "foo", "bar"))
		assert.Equal(t, `{"foo":"bar"}`, string(v.Pack()))
	})

	t.Run("preserves other members", func(t *testing.T) {
		v, _ := hujson.Parse([]byte(`{"a":1,"b":2}`))
		def := defFrom(v)
		obj := v.Value.(*hujson.Object)
		require.NoError(t, def.setMember(obj, "a", 99))
		packed := string(v.Pack())
		assert.Contains(t, packed, `99`)
		assert.Contains(t, packed, `"b":2`)
	})

	t.Run("replaces with complex value", func(t *testing.T) {
		v, _ := hujson.Parse([]byte(`{"x":null}`))
		def := defFrom(v)
		obj := v.Value.(*hujson.Object)
		require.NoError(t, def.setMember(obj, "x", []string{"a", "b"}))
		assert.Equal(t, `{"x":["a","b"]}`, string(v.Pack()))
	})
}

func TestDeleteMember(t *testing.T) {
	t.Run("removes existing key", func(t *testing.T) {
		v, _ := hujson.Parse([]byte(`{"a":"1","b":"2"}`))
		def := defFrom(v)
		obj := v.Value.(*hujson.Object)
		def.deleteMember(obj, "a")
		assert.Equal(t, `{"b":"2"}`, string(v.Pack()))
	})

	t.Run("no-op when key absent", func(t *testing.T) {
		v, _ := hujson.Parse([]byte(`{"a":"1"}`))
		def := defFrom(v)
		obj := v.Value.(*hujson.Object)
		def.deleteMember(obj, "missing")
		assert.Equal(t, `{"a":"1"}`, string(v.Pack()))
	})
}

func TestAppendElement(t *testing.T) {
	t.Run("appends to array", func(t *testing.T) {
		v, _ := hujson.Parse([]byte(`[{"x":1}]`))
		def := defFrom(v)
		arr := v.Value.(*hujson.Array)
		require.NoError(t, def.appendElement(arr, map[string]int{"x": 2}))
		assert.Len(t, arr.Elements, 2)
	})

	t.Run("appends string value", func(t *testing.T) {
		v, _ := hujson.Parse([]byte(`["a"]`))
		def := defFrom(v)
		arr := v.Value.(*hujson.Array)
		require.NoError(t, def.appendElement(arr, "b"))
		assert.Len(t, arr.Elements, 2)
		assert.Contains(t, string(v.Pack()), `"b"`)
	})
}

func TestWriteToFile(t *testing.T) {
	t.Run("creates parent dirs and ends with newline", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "a", "b", "f.json")
		def := &jsonDefinition{}
		def.ast, _ = hujson.Parse([]byte(`[]`))
		require.NoError(t, def.WriteToFile(path))

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, byte('\n'), data[len(data)-1])
	})

	t.Run("array preserves comments and formatting is idempotent", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "f.json")
		src, err := os.ReadFile(testdataJSONCArray)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, src, 0o644))

		def, err := loadJsonFile(path)
		require.NoError(t, err)
		require.NoError(t, def.WriteToFile(path))

		first, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Contains(t, string(first), "// comment", "comments must survive")

		// Write again: output must be stable.
		def2, err := loadJsonFile(path)
		require.NoError(t, err)
		require.NoError(t, def2.WriteToFile(path))
		second, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, first, second, "formatting must be idempotent")
	})

	t.Run("object preserves comments and formatting is idempotent", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "f.json")
		src, err := os.ReadFile(testdataJSONObject)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, src, 0o644))

		def, err := loadJsonFile(path)
		require.NoError(t, err)
		require.NoError(t, def.WriteToFile(path))

		first, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Contains(t, string(first), "// JSONC comment", "comments must survive")

		// Write again: output must be stable.
		def2, err := loadJsonFile(path)
		require.NoError(t, err)
		require.NoError(t, def2.WriteToFile(path))
		second, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, first, second, "formatting must be idempotent")
	})
}

func TestMemberKey(t *testing.T) {
	v, _ := hujson.Parse([]byte(`{"hello":"world"}`))
	def := defFrom(v)
	obj := v.Value.(*hujson.Object)
	assert.Equal(t, "hello", def.memberKey(obj.Members[0]))
}

func TestTasksJsonRoundtrip(t *testing.T) {
	src, err := os.ReadFile(testdataTasks)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "tasks.json")
	require.NoError(t, os.WriteFile(path, src, 0o644))

	def, err := loadJsonFile(path)
	require.NoError(t, err)

	arr := def.Array()
	require.NotNil(t, arr)

	// Find "mage: fmt" by label and patch its command.
	idx := def.findByLabel(arr, "mage: fmt")
	require.GreaterOrEqual(t, idx, 0)
	fmtObj := arr.Elements[idx].Value.(*hujson.Object)
	require.NoError(t, def.setMember(fmtObj, "command", "mage2"))

	// Delete args from "mage: test".
	idx = def.findByLabel(arr, "mage: test")
	require.GreaterOrEqual(t, idx, 0)
	testObj := arr.Elements[idx].Value.(*hujson.Object)
	def.deleteMember(testObj, "args")

	// Append a new entry.
	require.NoError(t, def.appendElement(arr, map[string]string{"label": "new", "command": "true"}))

	// Remove the "custom" entry by filtering.
	kept := arr.Elements[:0]
	for _, elem := range arr.Elements {
		if def.elemLabel(elem) != "custom" {
			kept = append(kept, elem)
		}
	}
	arr.Elements = kept

	require.NoError(t, def.WriteToFile(path))

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
