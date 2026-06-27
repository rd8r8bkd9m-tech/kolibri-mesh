package awg

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
)

// TLSMask handles TLS masking of packets
type TLSMask struct {
	sni string
}

// NewTLSMask creates a new TLS mask
func NewTLSMask(sni string) *TLSMask {
	return &TLSMask{sni: sni}
}

// MaskPacket masks a packet as TLS ClientHello
func (t *TLSMask) MaskPacket(packet []byte) ([]byte, error) {
	// Create fake TLS ClientHello
	clientHello := t.createClientHello()

	// Append actual packet after TLS header
	result := make([]byte, len(clientHello)+len(packet))
	copy(result, clientHello)
	copy(result[len(clientHello):], packet)

	return result, nil
}

// UnmaskPacket extracts actual packet from TLS-masked data
func (t *TLSMask) UnmaskPacket(data []byte) ([]byte, error) {
	// Find end of TLS ClientHello
	tlsEnd := t.findTLSEnd(data)
	if tlsEnd < 0 || tlsEnd >= len(data) {
		return nil, fmt.Errorf("invalid TLS mask")
	}

	return data[tlsEnd:], nil
}

// createClientHello creates a fake TLS 1.3 ClientHello
func (t *TLSMask) createClientHello() []byte {
	var buf bytes.Buffer

	// TLS Record Header
	buf.WriteByte(0x16)       // Content Type: Handshake
	buf.WriteByte(0x03)       // TLS 1.2 (for compatibility)
	buf.WriteByte(0x01)       // TLS 1.2

	// Handshake Header
	buf.WriteByte(0x01)       // Handshake Type: ClientHello

	// ClientHello length (placeholder)
	clientHelloStart := buf.Len()
	buf.Write([]byte{0x00, 0x00, 0x00}) // 3 bytes length

	// Client Version
	buf.Write([]byte{0x03, 0x03}) // TLS 1.2

	// Random (32 bytes)
	random := make([]byte, 32)
	rand.Read(random)
	buf.Write(random)

	// Session ID Length
	buf.WriteByte(0x00)

	// Cipher Suites
	cipherSuites := []uint16{
		0x1301, // TLS_AES_128_GCM_SHA256
		0x1302, // TLS_AES_256_GCM_SHA384
		0x1303, // TLS_CHACHA20_POLY1305_SHA256
	}
	binary.Write(&buf, binary.BigEndian, uint16(len(cipherSuites)*2))
	for _, cs := range cipherSuites {
		binary.Write(&buf, binary.BigEndian, cs)
	}

	// Compression Methods
	buf.WriteByte(0x01) // Length
	buf.WriteByte(0x00) // null compression

	// Extensions
	extensionsStart := buf.Len()
	buf.Write([]byte{0x00, 0x00}) // Extensions length placeholder

	// SNI Extension
	t.addSNIExtension(&buf)

	// Supported Groups Extension
	t.addSupportedGroups(&buf)

	// Key Share Extension
	t.addKeyShare(&buf)

	// Supported Versions Extension
	t.addSupportedVersions(&buf)

	// Update extensions length
	extensionsLen := buf.Len() - extensionsStart - 2
	buf.Bytes()[extensionsStart] = byte(extensionsLen >> 8)
	buf.Bytes()[extensionsStart+1] = byte(extensionsLen)

	// Update ClientHello length
	clientHelloLen := buf.Len() - clientHelloStart - 3
	buf.Bytes()[clientHelloStart] = byte(clientHelloLen >> 16)
	buf.Bytes()[clientHelloStart+1] = byte(clientHelloLen >> 8)
	buf.Bytes()[clientHelloStart+2] = byte(clientHelloLen)

	return buf.Bytes()
}

// addSNIExtension adds Server Name Indication extension
func (t *TLSMask) addSNIExtension(buf *bytes.Buffer) {
	sni := []byte(t.sni)

	// Extension type
	binary.Write(buf, binary.BigEndian, uint16(0x0000)) // server_name

	// Extension length
	extensionLen := 5 + len(sni)
	binary.Write(buf, binary.BigEndian, uint16(extensionLen))

	// Server Name List Length
	binary.Write(buf, binary.BigEndian, uint16(3+len(sni)))

	// Server Name Type
	buf.WriteByte(0x00) // host_name

	// Server Name Length
	binary.Write(buf, binary.BigEndian, uint16(len(sni)))

	// Server Name
	buf.Write(sni)
}

// addSupportedGroups adds supported groups extension
func (t *TLSMask) addSupportedGroups(buf *bytes.Buffer) {
	groups := []uint16{
		0x001D, // x25519
		0x0017, // secp256r1
	}

	binary.Write(buf, binary.BigEndian, uint16(0x000A)) // supported_groups
	binary.Write(buf, binary.BigEndian, uint16(2+len(groups)*2))
	binary.Write(buf, binary.BigEndian, uint16(len(groups)*2))
	for _, g := range groups {
		binary.Write(buf, binary.BigEndian, g)
	}
}

// addKeyShare adds key share extension
func (t *TLSMask) addKeyShare(buf *bytes.Buffer) {
	// Generate fake x25519 key
	key := make([]byte, 32)
	rand.Read(key)

	binary.Write(buf, binary.BigEndian, uint16(0x0033)) // key_share
	binary.Write(buf, binary.BigEndian, uint16(4+2+32)) // length
	binary.Write(buf, binary.BigEndian, uint16(2+32))   // client key share length
	binary.Write(buf, binary.BigEndian, uint16(0x001D))  // x25519
	binary.Write(buf, binary.BigEndian, uint16(32))      // key length
	buf.Write(key)
}

// addSupportedVersions adds supported versions extension
func (t *TLSMask) addSupportedVersions(buf *bytes.Buffer) {
	binary.Write(buf, binary.BigEndian, uint16(0x002B)) // supported_versions
	binary.Write(buf, binary.BigEndian, uint16(3))      // length
	buf.WriteByte(0x02)                                 // versions length
	binary.Write(buf, binary.BigEndian, uint16(0x0304)) // TLS 1.3
}

// findTLSEnd finds the end of TLS ClientHello
func (t *TLSMask) findTLSEnd(data []byte) int {
	if len(data) < 5 {
		return -1
	}

	// Check TLS record header
	if data[0] != 0x16 {
		return -1
	}

	// Get record length
	recordLen := int(data[3])<<8 | int(data[4])
	if recordLen+5 > len(data) {
		return -1
	}

	return recordLen + 5
}
