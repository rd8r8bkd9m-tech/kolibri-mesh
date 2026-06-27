// Package awg implements KolibriAWG protocol
// KolibriAWG is a WireGuard-based protocol with obfuscation,
// TLS masking, jitter, and padding to bypass DPI.
package awg

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"math/big"
	"time"
)

// KolibriAWG parameters
type Config struct {
	// Seeds for obfuscation (S1-S4)
	S1 []byte `json:"s1"`
	S2 []byte `json:"s2"`
	S3 []byte `json:"s3"`
	S4 []byte `json:"s4"`

	// Hashes for verification (H1-H4)
	H1 []byte `json:"h1"`
	H2 []byte `json:"h2"`
	H3 []byte `json:"h3"`
	H4 []byte `json:"h4"`

	// Jitter parameters
	JMin int `json:"jmin"` // Minimum jitter in ms
	JMax int `json:"jmax"` // Maximum jitter in ms
	JC   int `json:"jc"`   // Jitter count

	// Padding
	PaddingMax int `json:"padding_max"` // Maximum padding bytes

	// TLS masking
	TLSSNI string `json:"tls_sni"` // SNI for TLS masking
}

// DefaultConfig returns default KolibriAWG parameters
func DefaultConfig() *Config {
	return &Config{
		S1:         generateRandomBytes(32),
		S2:         generateRandomBytes(32),
		S3:         generateRandomBytes(32),
		S4:         generateRandomBytes(32),
		H1:         generateRandomBytes(32),
		H2:         generateRandomBytes(32),
		H3:         generateRandomBytes(32),
		H4:         generateRandomBytes(32),
		JMin:       5,
		JMax:       50,
		JC:         6,
		PaddingMax: 256,
		TLSSNI:     "cloudflare.com",
	}
}

// Obfuscator handles packet obfuscation
type Obfuscator struct {
	config *Config
	cipher cipher.Block
}

// NewObfuscator creates a new obfuscator
func NewObfuscator(config *Config) (*Obfuscator, error) {
	// Derive key from seeds
	key := deriveKey(config.S1, config.S2, config.S3, config.S4)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	return &Obfuscator{
		config: config,
		cipher: block,
	}, nil
}

// Obfuscate obfuscates a WireGuard packet
func (o *Obfuscator) Obfuscate(packet []byte) ([]byte, error) {
	// 1. Add padding
	padded := o.addPadding(packet)

	// 2. Encrypt header
	encrypted := o.encryptHeader(padded)

	// 3. Add jitter markers
	withJitter := o.addJitterMarkers(encrypted)

	return withJitter, nil
}

// Deobfuscate deobfuscates a packet back to WireGuard format
func (o *Obfuscator) Deobfuscate(packet []byte) ([]byte, error) {
	// 1. Remove jitter markers
	withoutJitter := o.removeJitterMarkers(packet)

	// 2. Decrypt header
	decrypted := o.decryptHeader(withoutJitter)

	// 3. Remove padding
	original := o.removePadding(decrypted)

	return original, nil
}

// addPadding adds random padding to the packet
func (o *Obfuscator) addPadding(packet []byte) []byte {
	paddingLen := randomInt(o.config.PaddingMax)
	if paddingLen == 0 {
		return packet
	}

	padding := make([]byte, paddingLen)
	rand.Read(padding)

	// Add padding length as last byte
	result := make([]byte, len(packet)+paddingLen+1)
	copy(result, packet)
	copy(result[len(packet):], padding)
	result[len(result)-1] = byte(paddingLen)

	return result
}

// removePadding removes padding from the packet
func (o *Obfuscator) removePadding(packet []byte) []byte {
	if len(packet) == 0 {
		return packet
	}

	paddingLen := int(packet[len(packet)-1])
	if paddingLen >= len(packet) {
		return packet
	}

	return packet[:len(packet)-paddingLen-1]
}

// encryptHeader encrypts the WireGuard header
func (o *Obfuscator) encryptHeader(packet []byte) []byte {
	if len(packet) < 4 {
		return packet
	}

	// Encrypt first 4 bytes (WireGuard type + reserved)
	encrypted := make([]byte, len(packet))
	copy(encrypted, packet)

	// XOR with derived stream
	stream := o.deriveStream(len(encrypted))
	for i := range encrypted {
		encrypted[i] ^= stream[i]
	}

	return encrypted
}

// decryptHeader decrypts the WireGuard header
func (o *Obfuscator) decryptHeader(packet []byte) []byte {
	// XOR is symmetric
	return o.encryptHeader(packet)
}

// addJitterMarkers adds jitter timing markers
func (o *Obfuscator) addJitterMarkers(packet []byte) []byte {
	// Add jitter count as first byte
	result := make([]byte, len(packet)+1)
	result[0] = byte(o.config.JC)
	copy(result[1:], packet)
	return result
}

// removeJitterMarkers removes jitter timing markers
func (o *Obfuscator) removeJitterMarkers(packet []byte) []byte {
	if len(packet) == 0 {
		return packet
	}
	return packet[1:]
}

// deriveKey derives encryption key from seeds
func deriveKey(s1, s2, s3, s4 []byte) []byte {
	h := sha256.New()
	h.Write(s1)
	h.Write(s2)
	h.Write(s3)
	h.Write(s4)
	return h.Sum(nil)
}

// deriveStream generates a pseudorandom stream
func (o *Obfuscator) deriveStream(length int) []byte {
	stream := make([]byte, length)
	block := make([]byte, aes.BlockSize)

	for i := 0; i < length; i += aes.BlockSize {
		// Use H1-H4 as IV
		copy(block, o.config.H1[:aes.BlockSize])
		o.cipher.Encrypt(block, block)
		copy(stream[i:], block)
	}

	return stream[:length]
}

// ApplyJitter applies network jitter
func ApplyJitter(min, max int) {
	if min <= 0 || max <= 0 || min > max {
		return
	}

	jitter := randomInt(max - min)
	duration := time.Duration(min+jitter) * time.Millisecond
	time.Sleep(duration)
}

// generateRandomBytes generates random bytes
func generateRandomBytes(n int) []byte {
	b := make([]byte, n)
	rand.Read(b)
	return b
}

// randomInt generates random int in [0, max)
func randomInt(max int) int {
	if max <= 0 {
		return 0
	}
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(max)))
	return int(n.Int64())
}

// GenerateConfig generates a new KolibriAWG config
func GenerateConfig() *Config {
	return &Config{
		S1:         generateRandomBytes(32),
		S2:         generateRandomBytes(32),
		S3:         generateRandomBytes(32),
		S4:         generateRandomBytes(32),
		H1:         generateRandomBytes(32),
		H2:         generateRandomBytes(32),
		H3:         generateRandomBytes(32),
		H4:         generateRandomBytes(32),
		JMin:       5,
		JMax:       50,
		JC:         6,
		PaddingMax: 256,
		TLSSNI:     "cloudflare.com",
	}
}

// ValidateConfig validates KolibriAWG config
func ValidateConfig(config *Config) error {
	if len(config.S1) != 32 || len(config.S2) != 32 ||
		len(config.S3) != 32 || len(config.S4) != 32 {
		return fmt.Errorf("seeds S1-S4 must be 32 bytes each")
	}
	if len(config.H1) != 32 || len(config.H2) != 32 ||
		len(config.H3) != 32 || len(config.H4) != 32 {
		return fmt.Errorf("hashes H1-H4 must be 32 bytes each")
	}
	if config.JMin < 0 || config.JMax < 0 || config.JMin > config.JMax {
		return fmt.Errorf("invalid jitter parameters: JMin=%d, JMax=%d", config.JMin, config.JMax)
	}
	if config.JC < 0 {
		return fmt.Errorf("jitter count must be non-negative")
	}
	if config.PaddingMax < 0 {
		return fmt.Errorf("padding max must be non-negative")
	}
	return nil
}
