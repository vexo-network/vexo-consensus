package address

import (
	"errors"
	"strings"
)

const bech32Charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

var (
	ErrInvalidBech32 = errors.New("invalid bech32")
	ErrMixedCase     = errors.New("mixed-case bech32")
)

func bech32Encode(hrp string, payload []byte) (string, error) {
	data, err := convertBits(payload, 8, 5, true)
	if err != nil {
		return "", err
	}
	checksum := createChecksum(hrp, data)
	combined := append(append([]byte(nil), data...), checksum...)
	var builder strings.Builder
	builder.Grow(len(hrp) + 1 + len(combined))
	builder.WriteString(hrp)
	builder.WriteByte('1')
	for _, value := range combined {
		if value >= byte(len(bech32Charset)) {
			return "", ErrInvalidBech32
		}
		builder.WriteByte(bech32Charset[value])
	}
	return builder.String(), nil
}

func bech32Decode(value string) (string, []byte, error) {
	if value == "" || strings.ToLower(value) != value && strings.ToUpper(value) != value {
		return "", nil, ErrMixedCase
	}
	value = strings.ToLower(value)
	if len(value) < 8 {
		return "", nil, ErrInvalidBech32
	}
	separator := strings.LastIndexByte(value, '1')
	if separator <= 0 || separator+7 > len(value) {
		return "", nil, ErrInvalidBech32
	}
	hrp := value[:separator]
	if err := validateHRP(hrp); err != nil {
		return "", nil, err
	}
	encoded := value[separator+1:]
	data := make([]byte, len(encoded))
	for index, char := range encoded {
		position := strings.IndexRune(bech32Charset, char)
		if position < 0 {
			return "", nil, ErrInvalidBech32
		}
		data[index] = byte(position)
	}
	if !verifyChecksum(hrp, data) {
		return "", nil, ErrInvalidBech32
	}
	payload, err := convertBits(data[:len(data)-6], 5, 8, false)
	if err != nil {
		return "", nil, err
	}
	return hrp, payload, nil
}

func createChecksum(hrp string, data []byte) []byte {
	values := append(hrpExpand(hrp), data...)
	values = append(values, 0, 0, 0, 0, 0, 0)
	polymod := bech32Polymod(values) ^ 1
	checksum := make([]byte, 6)
	for index := range checksum {
		checksum[index] = byte((polymod >> uint(5*(5-index))) & 31)
	}
	return checksum
}

func verifyChecksum(hrp string, data []byte) bool {
	return bech32Polymod(append(hrpExpand(hrp), data...)) == 1
}

func hrpExpand(hrp string) []byte {
	expanded := make([]byte, 0, len(hrp)*2+1)
	for _, char := range hrp {
		expanded = append(expanded, byte(char>>5))
	}
	expanded = append(expanded, 0)
	for _, char := range hrp {
		expanded = append(expanded, byte(char&31))
	}
	return expanded
}

func bech32Polymod(values []byte) uint32 {
	checksum := uint32(1)
	generators := []uint32{0x3b6a57b2, 0x26508e6d, 0x1ea119fa, 0x3d4233dd, 0x2a1462b3}
	for _, value := range values {
		top := checksum >> 25
		checksum = (checksum&0x1ffffff)<<5 ^ uint32(value)
		for index, generator := range generators {
			if (top>>uint(index))&1 == 1 {
				checksum ^= generator
			}
		}
	}
	return checksum
}

func convertBits(data []byte, fromBits uint, toBits uint, pad bool) ([]byte, error) {
	var accumulator uint
	var bits uint
	maxValue := uint((1 << toBits) - 1)
	maxAccumulator := uint((1 << (fromBits + toBits - 1)) - 1)
	converted := make([]byte, 0, len(data)*int(fromBits)/int(toBits))
	for _, value := range data {
		if uint(value)>>fromBits != 0 {
			return nil, ErrInvalidBech32
		}
		accumulator = ((accumulator << fromBits) | uint(value)) & maxAccumulator
		bits += fromBits
		for bits >= toBits {
			bits -= toBits
			converted = append(converted, byte((accumulator>>bits)&maxValue))
		}
	}
	if pad {
		if bits > 0 {
			converted = append(converted, byte((accumulator<<(toBits-bits))&maxValue))
		}
	} else if bits >= fromBits || ((accumulator<<(toBits-bits))&maxValue) != 0 {
		return nil, ErrInvalidBech32
	}
	return converted, nil
}
