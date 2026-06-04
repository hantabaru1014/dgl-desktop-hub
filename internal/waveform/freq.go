package waveform

// MapFreqToDevice は入力周波数 (10..1000) を Coyote V3 のデバイス波形周波数
// 単位 (10..240) へ換算する。Coyote V3 仕様の区分線形換算に従う。
//
//	10..100   -> そのまま
//	101..600  -> (x-100)/5 + 100
//	601..1000 -> (x-600)/10 + 200
//	それ以外  -> 10
func MapFreqToDevice(input uint16) uint8 {
	switch {
	case input >= 10 && input <= 100:
		return uint8(input)
	case input >= 101 && input <= 600:
		return uint8((input-100)/5 + 100)
	case input >= 601 && input <= 1000:
		return uint8((input-600)/10 + 200)
	default:
		return 10
	}
}

// MapFreqFromDevice はデバイス波形周波数単位 (10..240) を入力周波数 (10..1000)
// へ逆換算する。MapFreqToDevice の各区間の下端へ写す近似逆変換。
func MapFreqFromDevice(dev uint8) uint16 {
	switch {
	case dev >= 10 && dev <= 100:
		return uint16(dev)
	case dev >= 101 && dev <= 200:
		return (uint16(dev)-100)*5 + 100
	case dev >= 201 && dev <= 240:
		return (uint16(dev)-200)*10 + 600
	default:
		return 10
	}
}
