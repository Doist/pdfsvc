// Package bytesize provides int64 wrapper type to aid with printing byte sizes
// in human friendly format.
package bytesize

import "fmt"

// Bytes type implements fmt.Stringer interface printing number of bytes in
// human-friendly form
type Bytes int64

func (b Bytes) String() string {
	ub := b
	if b < 0 {
		ub = -b
	}
	switch {
	case ub >= EiB:
		return fmt.Sprintf("%.2fEiB", float64(b)/float64(EiB))
	case ub >= PiB:
		return fmt.Sprintf("%.2fPiB", float64(b)/float64(PiB))
	case ub >= TiB:
		return fmt.Sprintf("%.2fTiB", float64(b)/float64(TiB))
	case ub >= GiB:
		return fmt.Sprintf("%.2fGiB", float64(b)/float64(GiB))
	case ub >= MiB:
		return fmt.Sprintf("%.2fMiB", float64(b)/float64(MiB))
	case ub >= KiB:
		return fmt.Sprintf("%.2fKiB", float64(b)/float64(KiB))
	}
	return fmt.Sprintf("%dB", b)
}

// Constants for common sizes
const (
	B Bytes = 1 << (10 * iota)
	KiB
	MiB
	GiB
	TiB
	PiB
	EiB
)
