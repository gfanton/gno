package gnolang

import (
	"fmt"
	"testing"

	"github.com/gotuna/gotuna/test/assert"
	"github.com/stretchr/testify/require"
)

const (
	pkgpath      = "gno.land/r/test/testdata"
	testMaxAlloc = 1500 * 1000 * 1000
)

// TestTypedValueMarshal_Primitive tests marshaling of primitive types.
func TestTypedValueMarshalJSON_Primitive(t *testing.T) {
	cases := []struct {
		ValueRep string // Go representation
		ArgRep   string // string representation
	}{
		// Boolean
		{"true", "true"},
		{"false", "false"},

		// int types
		{"int(42)", `42`}, // Needs to be quoted for amino
		{"int8(42)", `42`},
		{"int16(42)", `42`},
		{"int32(42)", `42`},
		{"int64(42)", `42`}, // Needs to be quoted for amino

		// uint types
		{"uint(42)", `42`}, // Needs to be quoted for amino
		{"uint8(42)", `42`},
		{"uint16(42)", `42`},
		{"uint32(42)", `42`},
		{"uint64(42)", `42`}, // Needs to be quoted for amino

		// Float types // XXX: Require amino unsafe
		// {"float32(3.14)", "3.14"},
		// {"float64(3.14)", "3.14"},

		// String type
		{`"hello world"`, `"hello world"`},
	}

	// Create TypedValue marshaler
	// tvm := NewTypedValueMarshaler(nil)

	for _, tc := range cases {
		tc := tc
		t.Run(tc.ValueRep, func(t *testing.T) {
			m := NewMachine("testdata", nil)

			nn := MustParseFile("testdata.gno",
				fmt.Sprintf(`package testdata; var Value = %s`, tc.ValueRep))

			m.RunFiles(nn)
			m.RunDeclaration(ImportD("testdata", "testdata"))

			tps := m.Eval(Sel(Nx("testdata"), "Value"))
			require.Len(t, tps, 1)

			t.Run("Marshal", func(t *testing.T) {
				raw, err := tps[0].Marshal()
				require.NoError(t, err)
				assert.Equal(t, tc.ArgRep, string(raw))
			})

			// t.Run("Unmarshal", func(t *testing.T) {
			// 	var tv TypedValue

			// 	require.NoError(t, err)
			// })

		})
	}
}

// TestTypedValueMarshal_Array tests marshaling of array types.
func TestTypedValueMarshalJSON_Array(t *testing.T) {
	cases := []struct {
		ValueRep string // Go representation
		ArgRep   string // string representation
	}{
		{`[]bool{true, false}`, "[true,false]"},
		{`[]int{1, 2, 3, 4, 5}`, `[1,2,3,4,5]`},
		{`[]uint{1, 2, 3, 4, 5}`, `[1,2,3,4,5]`},
		{`[]string{"hello", "world"}`, `["hello","world"]`},

		// XXX: Amino
		// {`[]int{1, 2, 3, 4, 5}`, `["1","2","3","4","5"]`},
		// {`[]uint{1, 2, 3, 4, 5}`, `["1","2","3","4","5"]`},

		// XXX: not supported by amino
		// {`[]float32{1.1, 2.2, 3.3}`, `["1.1","2.2","3.3"]`},

		// XXX: base64 encoded data byte
	}

	// Create TypedValue marshaler
	// tvm := NewTypedValueMarshaler(nil)

	for _, tc := range cases {
		tc := tc
		t.Run(tc.ArgRep, func(t *testing.T) {
			m := NewMachine("testdata", nil)
			defer m.Release()

			nn := MustParseFile("testdata.gno",
				fmt.Sprintf(`package testdata; var Value = %s`, tc.ValueRep))

			m.RunFiles(nn)
			m.RunDeclaration(ImportD("testdata", "testdata"))

			tps := m.Eval(Sel(Nx("testdata"), "Value"))
			require.Len(t, tps, 1)
			tv := tps[0]

			t.Run("Marshal", func(t *testing.T) {
				raw, err := tv.Marshal()
				require.NoError(t, err)
				assert.Equal(t, tc.ArgRep, string(raw))
			})

			// t.Run("Unmarshal", func(t *testing.T) {
			// 	err := amino.UnmarshalJSON([]byte(tc.ArgRep), mv)
			// 	require.NoError(t, err)
			// })

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

func TestTypedValueMarshalJSON_Struct(t *testing.T) {
	cases := []struct {
		ValueRep string // string representation
		Expected string // string representation
	}{
		{
			`Simple{}`,
			`{"A":0,"B":"","C":false}`,
		},
		{
			`Simple{A:0, B:"",C:false}`,
			`{"A":0,"B":"","C":false}`,
		},
		{
			`Simple{A:42,B:"hello gno",C:true}`,
			`{"A":42,"B":"hello gno","C":true}`,
		},
		{
			`Tags{A:42,B:"hello gno",C:true}`,
			`{"valueA":42,"valueB":"hello gno","valueC":true}`,
		},
		{
			`Nested{A:43,B: &Simple{A:42,B:"hello gno",C:true}}`,
			`{"A":43,"B":{"A":42,"B":"hello gno","C":true}}`,
		},

		{`Unexported{A:42}`, `{"A":42}`},

		// XXX: amino
		// {
		// 	`Simple{}`,
		// 	`{"A":"0","B":"","C":false}`,
		// },
		// {
		// 	`Simple{"A":"0","B":"","C":false}`,
		// 	`{"A":"0","B":"","C":false}`,
		// },
		// {
		// 	`Simple{"A":"42","B":"hello gno","C":true}`,
		// 	`{"A":"42","B":"hello gno","C":true}`,
		// },
		// {
		// 	`Tag{"A":"42","B":"hello gno","C":true}`,
		// 	`{"valueA":"42","valueB":"hello gno","valueC":true}`,
		// },

		// Struct with unexported field
		// {"Unexported", `{"A":"42"}`, `{"A":"42"}`},

		// Struct with nested struct

		// XXX(FIXME): Interface arn't supported yet, here is a preview
		// on how it should works using proto like syntax
		// {"Interface", `{"A": "42", "I": {"@type": "/gno.StringValue", "value": "Hello"}}`},
	}
	// m.RunDeclaration(ImportD("testdata", pkgpath))

	// // Create TypedValue marshaler
	// tvm := NewTypedValueMarshaler(nil)

	for _, tc := range cases {
		tc := tc
		t.Run(tc.ValueRep, func(t *testing.T) {
			m := NewMachine(pkgpath, nil)
			defer m.Release()

			nn := MustParseFile("struct.gno", StructsFile)
			m.RunFiles(nn)
			nn = MustParseFile("testdata.gno",
				fmt.Sprintf(`package testdata; var Value = %s`, tc.ValueRep))
			m.RunFiles(nn)
			m.RunDeclaration(ImportD("testdata", pkgpath))

			tps := m.Eval(Sel(Nx("testdata"), "Value"))
			require.Len(t, tps, 1)
			tv := tps[0]

			t.Run("Marshal", func(t *testing.T) {
				raw, err := tv.Marshal()
				require.NoError(t, err)
				assert.Equal(t, tc.Expected, string(raw))
			})

			// t.Run("Unmarshal", func(t *testing.T) {
			// 	err := amino.UnmarshalJSON([]byte(tc.ArgRep), mv)
			// 	require.NoError(t, err)
			// })

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

// // TestTypedValueMarshal_RecursiveMarshalPanic tests marshaling of recursive structures.
func TestTypedValueMarshal_RecursiveMarshalPanic(t *testing.T) {
	m := NewMachine(pkgpath, nil)
	defer m.Release()

	nn := MustParseFile("testdata.gno", RecursiveValueFile)
	m.RunFiles(nn)
	m.RunDeclaration(ImportD("testdata", pkgpath))

	tps := m.Eval(Sel(Nx("testdata"), "RecursiveStruct"))
	require.Len(t, tps, 1)
	tv := tps[0]

	require.PanicsWithError(t,
		ErrRecursivePointer.Error(),
		func() {
			tv.Marshal()
		})
}
