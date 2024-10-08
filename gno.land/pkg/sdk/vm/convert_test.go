package vm

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/gnolang/gno/gnovm/pkg/gnolang"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertEmptyNumbers(t *testing.T) {
	tests := []struct {
		argT        gnolang.Type
		expectedErr string
	}{
		{gnolang.UintType, `error parsing uint "": strconv.ParseUint: parsing "": invalid syntax`},
		{gnolang.Uint64Type, `error parsing uint64 "": strconv.ParseUint: parsing "": invalid syntax`},
		{gnolang.Uint32Type, `error parsing uint32 "": strconv.ParseUint: parsing "": invalid syntax`},
		{gnolang.Uint16Type, `error parsing uint16 "": strconv.ParseUint: parsing "": invalid syntax`},
		{gnolang.Uint8Type, `error parsing uint8 "": strconv.ParseUint: parsing "": invalid syntax`},
		{gnolang.IntType, `error parsing int "": strconv.ParseInt: parsing "": invalid syntax`},
		{gnolang.Int64Type, `error parsing int64 "": strconv.ParseInt: parsing "": invalid syntax`},
		{gnolang.Int32Type, `error parsing int32 "": strconv.ParseInt: parsing "": invalid syntax`},
		{gnolang.Int16Type, `error parsing int16 "": strconv.ParseInt: parsing "": invalid syntax`},
		{gnolang.Int8Type, `error parsing int8 "": strconv.ParseInt: parsing "": invalid syntax`},
		{gnolang.Float64Type, `error parsing float64 "": parse mantissa: `},
		{gnolang.Float32Type, `error parsing float32 "": parse mantissa: `},
	}

	for _, tt := range tests {
		testname := fmt.Sprintf("%v", tt.argT)
		t.Run(testname, func(t *testing.T) {
			run := func() {
				_ = convertArgToGno("", tt.argT)
			}
			assert.PanicsWithValue(t, tt.expectedErr, run)
		})
	}
}

func TestConvertJSONValuePrimtive(t *testing.T) {
	cases := []struct {
		ValueRep string // Go representation
		Expected string // string representation
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

		// Float types
		{"float32(3.14)", "3.14"},
		{"float64(3.14)", "3.14"},

		// String type
		{`"hello world"`, `"hello world"`},
	}

	for _, tc := range cases {
		t.Run(tc.ValueRep, func(t *testing.T) {
			m := gnolang.NewMachine("testdata", nil)
			defer m.Release()

			nn := gnolang.MustParseFile("testdata.gno",
				fmt.Sprintf(`package testdata; var Value = %s`, tc.ValueRep))
			m.RunFiles(nn)
			m.RunDeclaration(gnolang.ImportD("testdata", "testdata"))

			tps := m.Eval(gnolang.Sel(gnolang.Nx("testdata"), "Value"))
			require.Len(t, tps, 1)

			tv := tps[0]

			rep := JSONPrimitiveValue(m, tv)
			require.Equal(t, tc.Expected, rep)
		})
	}
}

func TestConvertJSONValueStruct(t *testing.T) {
	const StructsFile = `
package testdata

// E struct, impement error
type E struct { S string }

func (e *E) Error() string { return e.S }
`

	t.Run("with pointer", func(t *testing.T) {
		m := gnolang.NewMachine("testdata", nil)
		defer m.Release()

		const expected = "Hello World"
		nn := gnolang.MustParseFile("struct.gno", StructsFile)
		m.RunFiles(nn)
		nn = gnolang.MustParseFile("testdata.gno",
			fmt.Sprintf(`package testdata; var Value = E{%q}`, expected))
		m.RunFiles(nn)
		m.RunDeclaration(gnolang.ImportD("testdata", "testdata"))

		tps := m.Eval(gnolang.Sel(gnolang.Nx("testdata"), "Value"))
		require.Len(t, tps, 1)

		tv := tps[0]
		rep := JSONPrimitiveValue(m, tv)
		require.Equal(t, strconv.Quote(expected), rep)
	})

	t.Run("without pointer", func(t *testing.T) {
		m := gnolang.NewMachine("testdata", nil)
		defer m.Release()

		const expected = "Hello World"
		nn := gnolang.MustParseFile("struct.gno", StructsFile)
		m.RunFiles(nn)
		nn = gnolang.MustParseFile("testdata.gno",
			fmt.Sprintf(`package testdata; var Value = &E{%q}`, expected))
		m.RunFiles(nn)
		m.RunDeclaration(gnolang.ImportD("testdata", "testdata"))

		tps := m.Eval(gnolang.Sel(gnolang.Nx("testdata"), "Value"))
		require.Len(t, tps, 1)

		tv := tps[0]
		rep := JSONPrimitiveValue(m, tv)
		require.Equal(t, strconv.Quote(expected), rep)
	})
}

func TestConvertJSONValuesList(t *testing.T) {
	cases := []struct {
		ValueRep []string // Go representation
		Expected string   // string representation
	}{
		// Boolean
		{[]string{}, "[]"},
		{[]string{"42"}, "[42]"},
		{[]string{"42", `"hello world"`}, `[42,"hello world"]`},
		{[]string{"42", `"hello world"`, "[]int{42}"}, `[42,"hello world"]`},
	}

	for _, tc := range cases {
		t.Run(strings.Join(tc.ValueRep, "-"), func(t *testing.T) {
			m := gnolang.NewMachine("testdata", nil)
			defer m.Release()

			nn := gnolang.MustParseFile("testdata.gno",
				fmt.Sprintf(`package testdata; var Value = []interface{}{%s}`, strings.Join(tc.ValueRep, ",")))
			m.RunFiles(nn)
			m.RunDeclaration(gnolang.ImportD("testdata", "testdata"))

			tps := m.Eval(gnolang.Sel(gnolang.Nx("testdata"), "Value"))
			require.Len(t, tps, 1)
			require.Equal(t, gnolang.SliceKind.String(), tps[0].T.Kind().String())
			tpvs := tps[0].V.(*gnolang.SliceValue).Base.(*gnolang.ArrayValue).List
			rep := JSONPrimitiveValues(m, tpvs)
			require.Equal(t, tc.Expected, rep)
		})
	}
}
