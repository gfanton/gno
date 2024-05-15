package gnoamino

import (
	"fmt"
	"testing"

	"github.com/gnolang/gno/gnovm/pkg/gnolang"
	"github.com/gnolang/gno/tm2/pkg/amino"
	"github.com/gotuna/gotuna/test/assert"
	"github.com/stretchr/testify/require"
)

const (
	pkgpath      = "gno.land/r/test/testdata"
	testMaxAlloc = 1500 * 1000 * 1000
)

// TestTypedValueMarshal_Primitive tests marshaling of primitive types.
func TestTypedValueMarshal_Primitive(t *testing.T) {
	cases := []struct {
		ValueRep string // Go representation
		ArgRep   string // string representation
	}{
		// Boolean
		{"true", "true"},
		{"false", "false"},

		// int types
		{"int(42)", `"42"`}, // Needs to be quoted for amino
		{"int8(42)", `42`},
		{"int16(42)", "42"},
		{"int32(42)", "42"},
		{"int64(42)", `"42"`}, // Needs to be quoted for amino

		// uint types
		{"uint(42)", `"42"`}, // Needs to be quoted for amino
		{"uint8(42)", "42"},
		{"uint16(42)", "42"},
		{"uint32(42)", "42"},
		{"uint64(42)", `"42"`}, // Needs to be quoted for amino

		// Float types // XXX: Require amino unsafe
		// {"float32(3.14)", "3.14"},
		// {"float64(3.14)", "3.14"},

		// String type
		{`"hello world"`, `"hello world"`},
	}

	// Create TypedValue marshaler
	tvm := NewTypedValueMarshaler(nil)

	for _, tc := range cases {
		tc := tc
		t.Run(tc.ValueRep, func(t *testing.T) {
			m := gnolang.NewMachine("testdata", nil)

			nn := gnolang.MustParseFile("testdata.gno",
				fmt.Sprintf(`package testdata; var Value = %s`, tc.ValueRep))

			m.RunFiles(nn)
			m.RunDeclaration(gnolang.ImportD("testdata", "testdata"))

			tps := m.Eval(gnolang.Sel(gnolang.Nx("testdata"), "Value"))
			require.Len(t, tps, 1)
			gt := tps[0].T

			// Create Marshaling type
			mv := tvm.From(gt)

			t.Run("Unmarshal", func(t *testing.T) {
				err := amino.UnmarshalJSON([]byte(tc.ArgRep), mv)
				require.NoError(t, err)
			})

			t.Run("Marshal", func(t *testing.T) {
				raw, err := amino.MarshalJSON(mv)
				require.NoError(t, err)
				assert.Equal(t, tc.ArgRep, string(raw))
			})

		})
	}
}

// TestTypedValueMarshal_Array tests marshaling of array types.
func TestTypedValueMarshal_Array(t *testing.T) {
	cases := []struct {
		ValueRep string // Go representation
		ArgRep   string // string representation
	}{
		{`[]bool{true, false}`, "[true,false]"},
		{`[]int{1, 2, 3, 4, 5}`, `["1","2","3","4","5"]`},
		{`[]uint{1, 2, 3, 4, 5}`, `["1","2","3","4","5"]`},
		{`[]string{"hello", "world"}`, `["hello","world"]`},

		// XXX: not supported by amino
		// {`[]float32{1.1, 2.2, 3.3}`, `["1.1","2.2","3.3"]`},

		// XXX: base64 encoded data byte
	}

	// Create TypedValue marshaler
	tvm := NewTypedValueMarshaler(nil)

	for _, tc := range cases {
		tc := tc
		t.Run(tc.ArgRep, func(t *testing.T) {
			m := gnolang.NewMachine("testdata", nil)
			defer m.Release()

			nn := gnolang.MustParseFile("testdata.gno",
				fmt.Sprintf(`package testdata; var Value = %s`, tc.ValueRep))

			m.RunFiles(nn)
			m.RunDeclaration(gnolang.ImportD("testdata", "testdata"))

			tps := m.Eval(gnolang.Sel(gnolang.Nx("testdata"), "Value"))
			require.Len(t, tps, 1)
			gt := tps[0].T

			// Create Marshaling type
			mv := tvm.From(gt)

			t.Run("Unmarshal", func(t *testing.T) {
				err := amino.UnmarshalJSON([]byte(tc.ArgRep), mv)
				require.NoError(t, err)
			})

			t.Run("Marshal", func(t *testing.T) {
				raw, err := amino.MarshalJSON(mv)
				require.NoError(t, err)
				assert.Equal(t, tc.ArgRep, string(raw))
			})

		})
	}
}

const StructsFile = `
package testdata

// Simple struct
type Simple struct {
	A int
	B string
	C bool
}

// Simple struct with tags
type Tags struct {
	A int ` + "`json:\"valueA\"`" + `
	B string ` + "`json:\"valueB\"`" + `
	C bool ` + "`json:\"valueC\"`" + `
}

// Struct with unexported field
type Unexported struct {
	A int
	b string
}

// Nested struct
type Nested struct {
	A int
	B *Simple
}

// Struct with an interface field
type Interface struct {
	A int
	I interface{}
}
`

func TestTypedValueMarshal_Struct(t *testing.T) {
	cases := []struct {
		ValueRepName string // Go representation
		ArgRep       string // string representation
		Expected     string // string representation
	}{
		// Struct with various field values.
		{"Simple",
			`{}`,
			`{"A":"0","B":"","C":false}`}, // empty struct
		{"Simple",
			`{"A":"0","B":"","C":false}`,
			`{"A":"0","B":"","C":false}`}, // empty value
		{"Simple",
			`{"A":"42","B":"hello gno","C":true}`,
			`{"A":"42","B":"hello gno","C":true}`}, // filled values
		{"Tags",
			`{"valueA":"42","valueB":"hello gno","valueC":true}`,
			`{"valueA":"42","valueB":"hello gno","valueC":true}`}, // filled values

		// Struct with unexported field
		{"Unexported", `{"A":"42"}`, `{"A":"42"}`},

		// Struct with nested struct
		{"Nested",
			`{"A":"43","B":{"A":"42","B":"hello gno","C":true}}`,
			`{"A":"43","B":{"A":"42","B":"hello gno","C":true}}`,
		},

		// XXX(FIXME): Interface arn't supported yet, here is a preview
		// on how it should works using proto like syntax
		// {"Interface", `{"A": "42", "I": {"@type": "/gno.StringValue", "value": "Hello"}}`},
	}

	m := gnolang.NewMachine(pkgpath, nil)
	defer m.Release()

	nn := gnolang.MustParseFile("testdata.gno", StructsFile)
	m.RunFiles(nn)
	m.RunDeclaration(gnolang.ImportD("testdata", pkgpath))

	// Create TypedValue marshaler
	tvm := NewTypedValueMarshaler(nil)

	for _, tc := range cases {
		tc := tc
		t.Run(tc.ArgRep, func(t *testing.T) {
			tps := m.Eval(gnolang.Sel(gnolang.Nx("testdata"), tc.ValueRepName))
			require.Len(t, tps, 1)
			gt := tps[0].V.(gnolang.TypeValue).Type

			// Create Marshaling type
			mv := tvm.From(gt)

			t.Run("Unmarshal", func(t *testing.T) {
				err := amino.UnmarshalJSON([]byte(tc.ArgRep), mv)
				require.NoError(t, err)
			})

			t.Run("Marshal", func(t *testing.T) {
				raw, err := amino.MarshalJSON(mv)
				require.NoError(t, err)
				assert.Equal(t, tc.Expected, string(raw))
			})

		})
	}
}

const RecursiveValueFile = `
package testdata

type Recursive struct {
	Nested *Recursive
}

var RecursiveStruct = &Recursive{}

func init() {
	RecursiveStruct.Nested = RecursiveStruct
}
`

// TestTypedValueMarshal_RecursiveMarshalPanic tests marshaling of recursive structures.
func TestTypedValueMarshal_RecursiveMarshalPanic(t *testing.T) {
	m := gnolang.NewMachine(pkgpath, nil)
	defer m.Release()

	nn := gnolang.MustParseFile("testdata.gno", RecursiveValueFile)
	m.RunFiles(nn)
	m.RunDeclaration(gnolang.ImportD("testdata", pkgpath))

	tps := m.Eval(gnolang.Sel(gnolang.Nx("testdata"), "RecursiveStruct"))
	require.Len(t, tps, 1)
	gv := tps[0]

	// Create a TypedValue marshaler
	tvm := NewTypedValueMarshaler(nil)
	mv := tvm.Wrap(&gv)

	require.PanicsWithError(t,
		ErrRecursivePointer.Error(),
		func() { amino.MarshalJSON(mv) },
	)
}
