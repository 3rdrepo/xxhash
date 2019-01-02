// Package xxhash implements the 64-bit variant of xxHash (XXH64) as described
// at http://cyan4973.github.io/xxHash/.
package xxhash

import (
	"encoding/binary"
)

const (
	prime1 uint64 = 11400714785074694791
	prime2 uint64 = 14029467366897019727
	prime3 uint64 = 1609587929392839161
	prime4 uint64 = 9650029242287828579
	prime5 uint64 = 2870177450012600261
)

// NOTE(caleb): I'm using both consts and vars of the primes. Using consts where
// possible in the Go code is worth a small (but measurable) performance boost
// by avoiding some MOVQs. Vars are needed for the asm and also are useful for
// convenience in the Go code in a few places where we need to intentionally
// avoid constant arithmetic (e.g., v1 := prime1 + prime2 fails because the
// result overflows a uint64).
var (
	prime1v = prime1
	prime2v = prime2
	prime3v = prime3
	prime4v = prime4
	prime5v = prime5
)

// Sum64 computes the 64-bit xxHash digest of b.
func Sum64(b []byte) uint64 {
	return sum64(b)
}

// Sum64String computes the 64-bit xxHash digest of s.
// It may be faster than Sum64([]byte(s)) by avoiding a copy.
func Sum64String(s string) uint64 {
	return sum64String(s)
}

// Digest implements hash.Hash64.
type Digest struct {
	v1    uint64
	v2    uint64
	v3    uint64
	v4    uint64
	total int
	mem   [32]byte
	n     int // how much of mem is used
}

// New creates a new Digest that computes the 64-bit xxHash algorithm.
func New() *Digest {
	var d Digest
	d.Reset()
	return &d
}

// Reset clears the Digest's state so that it can be reused.
func (d *Digest) Reset() {
	d.n = 0
	d.total = 0
	d.v1 = prime1v + prime2
	d.v2 = prime2
	d.v3 = 0
	d.v4 = -prime1v
}

// Size always returns 8 bytes.
func (d *Digest) Size() int { return 8 }

// BlockSize always returns 32 bytes.
func (d *Digest) BlockSize() int { return 32 }

// Write adds more data to d. It always returns len(b), nil.
func (d *Digest) Write(b []byte) (n int, err error) {
	n = len(b)
	d.total += len(b)

	if d.n+len(b) < 32 {
		// This new data doesn't even fill the current block.
		copy(d.mem[d.n:], b)
		d.n += len(b)
		return
	}

	if d.n > 0 {
		// Finish off the partial block.
		copy(d.mem[d.n:], b)
		d.v1 = round(d.v1, u64(d.mem[0:8]))
		d.v2 = round(d.v2, u64(d.mem[8:16]))
		d.v3 = round(d.v3, u64(d.mem[16:24]))
		d.v4 = round(d.v4, u64(d.mem[24:32]))
		b = b[32-d.n:]
		d.n = 0
	}

	if len(b) >= 32 {
		// One or more full blocks left.
		nw := writeBlocks(d, b)
		b = b[nw:]
	}

	// Store any remaining partial block.
	copy(d.mem[:], b)
	d.n = len(b)

	return
}

// WriteString adds more data to d. It always returns len(s), nil.
// It may be faster than Write([]byte(s)) by avoiding a copy.
func (d *Digest) WriteString(s string) (n int, err error) {
	return d.writeString(s)
}

// Sum appends the current hash to b and returns the resulting slice.
func (d *Digest) Sum(b []byte) []byte {
	s := d.Sum64()
	return append(
		b,
		byte(s>>56),
		byte(s>>48),
		byte(s>>40),
		byte(s>>32),
		byte(s>>24),
		byte(s>>16),
		byte(s>>8),
		byte(s),
	)
}

// Sum64 returns the current hash.
func (d *Digest) Sum64() uint64 {
	var h uint64

	if d.total >= 32 {
		v1, v2, v3, v4 := d.v1, d.v2, d.v3, d.v4
		h = rol1(v1) + rol7(v2) + rol12(v3) + rol18(v4)
		h = mergeRound(h, v1)
		h = mergeRound(h, v2)
		h = mergeRound(h, v3)
		h = mergeRound(h, v4)
	} else {
		h = d.v3 + prime5
	}

	h += uint64(d.total)

	i, end := 0, d.n
	for ; i+8 <= end; i += 8 {
		k1 := round(0, u64(d.mem[i:i+8]))
		h ^= k1
		h = rol27(h)*prime1 + prime4
	}
	if i+4 <= end {
		h ^= uint64(u32(d.mem[i:i+4])) * prime1
		h = rol23(h)*prime2 + prime3
		i += 4
	}
	for i < end {
		h ^= uint64(d.mem[i]) * prime5
		h = rol11(h) * prime1
		i++
	}

	h ^= h >> 33
	h *= prime2
	h ^= h >> 29
	h *= prime3
	h ^= h >> 32

	return h
}

func u64(b []byte) uint64 { return binary.LittleEndian.Uint64(b) }
func u32(b []byte) uint32 { return binary.LittleEndian.Uint32(b) }

func round(acc, input uint64) uint64 {
	acc += input * prime2
	acc = rol31(acc)
	acc *= prime1
	return acc
}

func mergeRound(acc, val uint64) uint64 {
	val = round(0, val)
	acc ^= val
	acc = acc*prime1 + prime4
	return acc
}
