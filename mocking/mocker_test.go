package mocking

import (
	"bytes"
	_ "embed"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	//go:embed test_files/predefined_named.input
	predefinedNamedInterface string
	//go:embed test_files/predefined_named.output
	predefinedNamedResult string
	//go:embed test_files/custom_named.input
	customNamedInterface string
	//go:embed test_files/custom_named.output
	customNamedResult string
	//go:embed test_files/time_return_slice.input
	timeSliceInterface string
	//go:embed test_files/time_return_slice.output
	timeSliceResult string
	//go:embed test_files/pointer_slice.input
	pointerSliceInterface string
	//go:embed test_files/pointer_slice.output
	pointerSliceResult string
)

func TestMocker_Success(t *testing.T) {
	tests := map[string]struct {
		in             string
		expectedResult string
		intfName       string
	}{
		"predefined_named": {
			in:             predefinedNamedInterface,
			expectedResult: predefinedNamedResult,
			intfName:       "PredNamed",
		},
		"custom_named": {
			in:             customNamedInterface,
			expectedResult: customNamedResult,
			intfName:       "CustomNamed",
		},
		"time&slice": {
			in:             timeSliceInterface,
			expectedResult: timeSliceResult,
			intfName:       "TimeSlice",
		},
		"pointer_slice": {
			in:             pointerSliceInterface,
			expectedResult: pointerSliceResult,
			intfName:       "PointerSlice",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			mocker := NewMocker()
			var b bytes.Buffer
			err := mocker.Mock(tc.in, &b, tc.intfName)
			require.Nil(t, err)
			require.Equal(t, tc.expectedResult, b.String())
		})
	}

}
