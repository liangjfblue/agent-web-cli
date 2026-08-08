package proto

// crc16 computes CRC-16/CCITT-FALSE over data.
// Polynomial 0x1021, init 0xFFFF, no reflection, xorout 0x0000.
// Used purely as a frame integrity check against half-read or concatenated
// payloads; it is not a security mechanism.
func crc16(data []byte) uint16 {
	const poly = 0x1021
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ poly
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}
