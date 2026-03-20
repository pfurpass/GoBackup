package core

// BackupMagic identifies a valid backup file
const BackupMagic = "GOBACKUP"

// FileVersion current format version
const FileVersion = uint32(1)

// Flags for backup options
const (
	FlagCompressed = uint32(1 << 0) // zstd compression enabled
	FlagEncrypted  = uint32(1 << 1) // AES-256-GCM encryption enabled
)

// DefaultChunkSize is 1MB
const DefaultChunkSize = 1024 * 1024

// HeaderSize is the fixed size of the binary header in bytes
const HeaderSize = 512

// ChunkHeaderSize is the fixed size of each chunk header
const ChunkHeaderSize = 52

// BackupHeader is written at the start of every backup file.
// Total size must equal HeaderSize (512 bytes).
//
// Layout (all little-endian):
//   [0:8]   Magic       [8]byte
//   [8:12]  Version     uint32
//   [12:16] Flags       uint32
//   [16:272] DevicePath [256]byte
//   [272:280] DevSize   uint64
//   [280:284] ChunkSize uint32
//   [284:316] Salt      [32]byte  (PBKDF2 salt; zeros if no encryption)
//   [316:348] DataHash  [32]byte  (SHA-256 of ALL original uncompressed bytes)
//   [348:512] Reserved  [164]byte
type BackupHeader struct {
	Magic      [8]byte
	Version    uint32
	Flags      uint32
	DevicePath [256]byte
	DeviceSize uint64
	ChunkSize  uint32
	Salt       [32]byte
	DataHash   [32]byte
	Reserved   [164]byte
}

// ChunkHeader precedes every data chunk in the backup file.
//
// Layout:
//   [0:4]   OrigSize  uint32   bytes before compression/encryption
//   [4:8]   StoreSize uint32   bytes actually written (after comp+enc)
//   [8:20]  Nonce     [12]byte AES-GCM nonce (zeros when not encrypted)
//   [20:52] ChunkHash [32]byte SHA-256 of the original (pre-comp) chunk bytes
type ChunkHeader struct {
	OrigSize  uint32
	StoreSize uint32
	Nonce     [12]byte
	ChunkHash [32]byte
}
