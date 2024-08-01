package gnolang

import (
	"fmt"
	"testing"

	"github.com/gnolang/gno/tm2/pkg/std"
	"github.com/gotuna/gotuna/test/assert"
	"github.com/stretchr/testify/require"
)

const (
	pkgpath      = "gno.land/r/test/testdata"
	testMaxAlloc = 1500 * 1000 * 1000
)

// TestTypedValueMarshal_Primitive tests marshaling of primitive types.
func TestTypedValueJSON_Primitive(t *testing.T) {
	cases := []struct {
		ValueRep string // Go representation
		ArgRep   string // string representation
	}{
		// Boolean
		{"nil", "null"},

		// Boolean
		{"true", "true"},
		{"false", "false"},

		// int types
		{"int(42)", `42`}, // Needs to be quoted for amino
		{"int8(42)", `42`},
		{"int16(42)", `42`},
		{"int32(42)", `42`},
		{"int64(42)", `42`},

		// uint types
		{"uint(42)", `42`},
		{"uint8(42)", `42`},
		{"uint16(42)", `42`},
		{"uint32(42)", `42`},
		{"uint64(42)", `42`},

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

			tv := tps[0]
			t.Run("Marshal", func(t *testing.T) {
				raw, err := tps[0].MarshalJSON()
				require.NoError(t, err)
				assert.Equal(t, tc.ArgRep, string(raw))
			})

			t.Run("Unmarshal", func(t *testing.T) {
				var utv TypedValue

				// copy type
				utv.T = tv.T

				err := utv.UnmarshalJSON([]byte(tc.ArgRep))
				require.NoError(t, err)

				require.Equal(t, tv.String(), utv.String())
			})
		})
	}
}

// TestTypedValueMarshal_Array tests marshaling of array types.
func TestTypedValueJSON_Slice(t *testing.T) {
	cases := []struct {
		ValueRep string // Go representation
		ArgRep   string // string representation
	}{
		{`[]bool{true, false}`, "[true,false]"},
		{`[]int{1, 2, 3, 4, 5}`, `[1,2,3,4,5]`},
		{`[]uint{1, 2, 3, 4, 5}`, `[1,2,3,4,5]`},
		{`[]string{"hello", "world"}`, `["hello","world"]`},
		{
			`[]interface{}{"hello", 32, true, struct{A string}{"high"}}`,
			`["hello",32,true,{"A":"high"}]`,
		},

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
				raw, err := tv.MarshalJSON()
				require.NoError(t, err)
				assert.Equal(t, tc.ArgRep, string(raw))
			})

			t.Run("Unmarshal", func(t *testing.T) {
				var utv TypedValue
				fmt.Println(tv.String())
				// copy type
				utv.T = tv.T

				err := utv.UnmarshalJSON([]byte(tc.ArgRep))
				require.NoError(t, err)

				require.Equal(t, tv.String(), utv.String())
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
	C bool ` + "`json:\"valueC,omitempty\"`" + `
	D *Simple ` + "`json:\"valueD,omitempty\"`" + `
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

func TestTypedValueJSONMarshal_Struct(t *testing.T) {
	cases := []struct {
		GnoValue      string // s tring representation
		Expected      string // string representation
		ExpectedAmino string // string representation
	}{
		{
			`Simple{}`,
			`{"A":0,"B":"","C":false}`,
			`{"A":"0","B":"","C":false}`,
		},
		{
			`Simple{A:0, B:"",C:false}`,
			`{"A":0,"B":"","C":false}`,
			`{"A":"0","B":"","C":false}`,
		},
		{
			`Simple{A:42,B:"hello gno",C:true}`,
			`{"A":42,"B":"hello gno","C":true}`,
			`{"A":"42","B":"hello gno","C":true}`,
		},
		{
			`Simple{A:42,B:"hello gno",C:true}`,
			`{"A":42,"B":"hello gno","C":true}`,
			`{"A":"42","B":"hello gno","C":true}`,
		},

		// Tags
		{
			// Tags filled
			`Tags{A:42, B:"hello gno",C:true,D:&Simple{}}`,
			`{"valueA":42,"valueB":"hello gno","valueC":true,"valueD":{"A":0,"B":"","C":false}}`,
			`{"valueA":"42","valueB":"hello gno","valueC":true,"valueD":{"A":0,"B":"","C":false}}`,
		},
		{
			// Tags ommitempty
			`Tags{A:0,B:"",C:false,D:nil}`,
			`{"valueA":0,"valueB":""}`,
			`{"valueA":0,"valueB":""}`,
		},
		{
			// Tags emtpy struct
			`Tags{}`,
			`{"valueA":0,"valueB":""}`,
			`{"valueA":0,"valueB":""}`,
		},

		// Nested
		{
			`Nested{A:43,B: &Simple{A:42,B:"hello gno",C:true}}`,
			`{"A":43,"B":{"A":42,"B":"hello gno","C":true}}`,
			`{"A":"43","B":{"A":42,"B":"hello gno","C":true}}`,
		},

		// Interface
		{
			`Interface{A:42, I: nil}`,
			`{"A":42,"I":null}`,
			`{"A":"42","I":null}`,
		},

		{
			`Interface{A:42, I: &Simple{A: 42}}`,
			`{"A":42,"I":{"A":42,"B":"","C":false}}`,
			`{"A":"42","I":{"A":"42","B":"","C":false}}`,
		},

		// Unexported
		{`Unexported{A:42}`, `{"A":42}`, `{"A":"42"}`},

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
		t.Run(tc.GnoValue, func(t *testing.T) {
			m := NewMachine(pkgpath, nil)
			defer m.Release()

			nn := MustParseFile("struct.gno", StructsFile)
			m.RunFiles(nn)
			nn = MustParseFile("testdata.gno",
				fmt.Sprintf(`package testdata; var Value = %s`, tc.GnoValue))
			m.RunFiles(nn)
			m.RunDeclaration(ImportD("testdata", pkgpath))

			tps := m.Eval(Sel(Nx("testdata"), "Value"))
			require.Len(t, tps, 1)
			tv := tps[0]

			t.Run("Marshal", func(t *testing.T) {
				raw, err := tv.MarshalJSON()
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

func TestTypedValueJSONUnmarshal_Struct(t *testing.T) {
	cases := []struct {
		GnoExpectedValue string // Expected value represation after unmarshal
		JSONValue        string // json string representation
		JSONAminoValue   string // json amino string representation
	}{
		{
			`(*Simple)(nil)`,
			`null`,
			`null`,
		},
		{
			`Simple{}`,
			`{}`,
			`{}`,
		},
		{
			`Simple{A:0, B:"",C:false}`,
			`{"A":0,"B":"","C":false}`,
			`{"A":"0","B":"","C":false}`,
		},
		{
			// Random postion
			`Simple{A:42,B:"hello gno",C:true}`,
			`{"C":true, "B":"hello gno", "A":42}`,
			`{"C":true, "B":"hello gno", "A":"42"}`,
		},

		// Nested
		{
			`Nested{A:43,B: &Simple{A:42,B:"hello gno",C:true}}`,
			`{"A":43,"B":{"A":42,"B":"hello gno","C":true}}`,
			`{"A":"43","B":{"A":42,"B":"hello gno","C":true}}`,
		},
		{
			`Nested{A:43,B: &Simple{}}`,
			`{"A":43,"B":{}}`,
			`{"A":"43","B":{}}`,
		},
		{
			`Nested{A:43,B: nil}`,
			`{"A":43, "B": null}`,
			`{"A":"43", "B": null}`,
		},
		{
			`Nested{A:43,B: nil}`,
			`{"A":43}`,
			`{"A":"43"}`,
		},
		{
			`Nested{A:43,B: &Simple{A:42,B:"hello gno",C:true}}`,
			`{"A":43,"B":{"A":42,"B":"hello gno","C":true}}`,
			`{"A":"43","B":{"A":42,"B":"hello gno","C":true}}`,
		},

		// Tags
		{
			// Tags filled
			`Tags{A:42, B:"hello gno",C:true,D:&Simple{B: ""}}`,
			`{"valueA":42,"valueB":"hello gno","valueC":true,"valueD":{"A":0,"B":"","C":false}}`,
			`{"valueA":"42","valueB":"hello gno","valueC":true,"valueD":{"A":0,"B":"","C":false}}`,
		},
		{
			// Tags ommitempty
			`Tags{A:0,B:"",C:false,D:nil}`,
			`{"valueA":0,"valueB":""}`,
			`{"valueA":0,"valueB":""}`,
		},

		// Interface
		{
			`Interface{A:42, I: nil}`,
			`{"A":42,"I":null}`,
			`{"A":"42","I":null}`,
		},

		{
			`Interface{A:42, I: 43}`,
			`{"A":42,"I":43}`,
			`{"A":42,"I":43}`,
		},

		// {
		// 	`Interface{A:42, I: &Simple{A: 42}}`,
		// 	`{"A":42,"I":{"A":42,"B":"","C":false}}`,
		// 	`{"A":"42","I":{"A":"42","B":"","C":false}}`,
		// },

		// Unexported
		// {`Unexported{A:42}`, `{"A":42}`, `{"A":"42"}`},

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

	// XXX: REMOVE ME, TESTING BLOCK
	// m := NewMachine(pkgpath, nil)
	// defer m.Release()

	// nn := MustParseFile("struct.gno", StructsFile)
	// m.RunFiles(nn)
	// nn = MustParseFile("testdata.gno",
	// 	fmt.Sprintf(`package testdata; var Value interface{} = 2`))
	// m.RunFiles(nn)
	// m.RunDeclaration(ImportD("testdata", pkgpath))

	// tps := m.Eval(Sel(Nx("testdata"), "Value"))
	// require.Len(t, tps, 1)
	// val := tps[0]
	// fmt.Printf("typ: %s val: %#v\n", val.T.String(), val)
	// panic("stop")

	for _, tc := range cases {
		tc := tc
		t.Run(tc.GnoExpectedValue, func(t *testing.T) {
			m := NewMachine(pkgpath, nil)
			defer m.Release()

			nn := MustParseFile("struct.gno", StructsFile)
			m.RunFiles(nn)
			nn = MustParseFile("testdata.gno",
				fmt.Sprintf(`package testdata; var Value = %s`, tc.GnoExpectedValue))
			m.RunFiles(nn)
			m.RunDeclaration(ImportD("testdata", pkgpath))

			tps := m.Eval(Sel(Nx("testdata"), "Value"))
			require.Len(t, tps, 1)
			expected := tps[0]

			// t.Run("Marshal", func(t *testing.T) {
			// 	raw, err := tv.MarshalJSON()
			// 	require.NoError(t, err)
			// 	assert.Equal(t, tc.Expected, string(raw))
			// })

			var utv TypedValue
			// copy type
			utv.T = expected.T

			err := utv.UnmarshalJSON([]byte(tc.JSONValue))
			require.NoError(t, err)

			require.Equal(t, expected.String(), utv.String())

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

// TestTypedValueMarshal_RecursiveMarshalPanic tests marshaling of recursive structures.
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
			tv.MarshalJSON()
		})
}

const RefValueFile = `
package refvalue

import "testdata"

var Value = testdata.Simple{}
`

func TestTypedValueMarshal_RefValue(t *testing.T) {
	m := NewMachine(pkgpath, nil)
	defer m.Release()

	sf := std.MemFile{
		Name: "struct.gno",
		Body: StructsFile,
	}
	m.RunMemPackage(&std.MemPackage{
		Name:  "testdata",
		Path:  "testdata",
		Files: []*std.MemFile{&sf},
	}, false)

	rf := std.MemFile{
		Name: "ref.gno",
		Body: RefValueFile,
	}
	m.RunMemPackage(&std.MemPackage{
		Name:  "refvalue",
		Path:  "refvalue",
		Files: []*std.MemFile{&rf},
	}, false)

	// sn := MustParseFile("structs.gno", StructsFile)
	m.RunDeclaration(ImportD("refvalue", "refvalue"))

	tps := m.Eval(Sel(Nx("refvalue"), "Value"))
	require.Len(t, tps, 1)
	tv := tps[0]

	raw, err := tv.MarshalJSON()
	require.NoError(t, err)
	println(string(raw))

}
