package bech32

import (
	"fmt"

	"github.com/btcsuite/btcd/btcutil/bech32"
)

// ConvertAndEncode encodes []byte to bech32.
// DEPRECATED use Encode
func ConvertAndEncode(hrp string, data []byte) (string, error) {
	converted, err := bech32.ConvertBits(data, 8, 5, true)
	if err != nil {
		return "", fmt.Errorf("encoding bech32 failed: %w", err)
	}
	return bech32.Encode(hrp, converted)
}

func Encode(hrp string, data []byte) (string, error) {
	return ConvertAndEncode(hrp, data)
}

// DecodeAndConvert decodes bech32 to []byte.
// DEPRECATED use Decode
func DecodeAndConvert(bech string) (string, []byte, error) {
	hrp, data, err := bech32.DecodeNoLimit(bech)
	if err != nil {
		return "", nil, fmt.Errorf("decoding bech32 failed: %w", err)
	}
	converted, err := bech32.ConvertBits(data, 5, 8, false)
	if err != nil {
		return "", nil, fmt.Errorf("decoding bech32 failed: %w", err)
	}
	return hrp, converted, nil
}

func Decode(bech string) (string, []byte, error) {
	return DecodeAndConvert(bech)
}
