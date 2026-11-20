package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math/bits"
)

// Pure-Go RFC 7914 scrypt implementation (zero external dependencies),
// bit-for-bit compatible with Node.js crypto.scryptSync(password, salt, 64)
// which uses default parameters N=16384, r=8, p=1, keyLen=64.

func pbkdf2SHA256(password, salt []byte, iter, keyLen int) []byte {
	prf := hmac.New(sha256.New, password)
	hashLen := prf.Size()
	numBlocks := (keyLen + hashLen - 1) / hashLen

	var buf [4]byte
	dk := make([]byte, 0, numBlocks*hashLen)
	u := make([]byte, hashLen)
	for block := 1; block <= numBlocks; block++ {
		prf.Reset()
		prf.Write(salt)
		buf[0] = byte(block >> 24)
		buf[1] = byte(block >> 16)
		buf[2] = byte(block >> 8)
		buf[3] = byte(block)
		prf.Write(buf[:4])
		dk = prf.Sum(dk)
		t := dk[len(dk)-hashLen:]
		copy(u, t)

		for n := 2; n <= iter; n++ {
			prf.Reset()
			prf.Write(u)
			u = u[:0]
			u = prf.Sum(u)
			for x := range u {
				t[x] ^= u[x]
			}
		}
	}
	return dk[:keyLen]
}

func salsaXOR(tmp *[16]uint32, inp, out []uint32) {
	w0 := tmp[0] ^ inp[0]
	w1 := tmp[1] ^ inp[1]
	w2 := tmp[2] ^ inp[2]
	w3 := tmp[3] ^ inp[3]
	w4 := tmp[4] ^ inp[4]
	w5 := tmp[5] ^ inp[5]
	w6 := tmp[6] ^ inp[6]
	w7 := tmp[7] ^ inp[7]
	w8 := tmp[8] ^ inp[8]
	w9 := tmp[9] ^ inp[9]
	w10 := tmp[10] ^ inp[10]
	w11 := tmp[11] ^ inp[11]
	w12 := tmp[12] ^ inp[12]
	w13 := tmp[13] ^ inp[13]
	w14 := tmp[14] ^ inp[14]
	w15 := tmp[15] ^ inp[15]

	x0, x1, x2, x3, x4, x5, x6, x7, x8 := w0, w1, w2, w3, w4, w5, w6, w7, w8
	x9, x10, x11, x12, x13, x14, x15 := w9, w10, w11, w12, w13, w14, w15

	for i := 0; i < 8; i += 2 {
		x4 ^= bits.RotateLeft32(x0+x12, 7)
		x8 ^= bits.RotateLeft32(x4+x0, 9)
		x12 ^= bits.RotateLeft32(x8+x4, 13)
		x0 ^= bits.RotateLeft32(x12+x8, 18)

		x9 ^= bits.RotateLeft32(x5+x1, 7)
		x13 ^= bits.RotateLeft32(x9+x5, 9)
		x1 ^= bits.RotateLeft32(x13+x9, 13)
		x5 ^= bits.RotateLeft32(x1+x13, 18)

		x14 ^= bits.RotateLeft32(x10+x6, 7)
		x2 ^= bits.RotateLeft32(x14+x10, 9)
		x6 ^= bits.RotateLeft32(x2+x14, 13)
		x10 ^= bits.RotateLeft32(x6+x2, 18)

		x3 ^= bits.RotateLeft32(x15+x11, 7)
		x7 ^= bits.RotateLeft32(x3+x15, 9)
		x11 ^= bits.RotateLeft32(x7+x3, 13)
		x15 ^= bits.RotateLeft32(x11+x7, 18)

		x1 ^= bits.RotateLeft32(x0+x3, 7)
		x2 ^= bits.RotateLeft32(x1+x0, 9)
		x3 ^= bits.RotateLeft32(x2+x1, 13)
		x0 ^= bits.RotateLeft32(x3+x2, 18)

		x6 ^= bits.RotateLeft32(x5+x4, 7)
		x7 ^= bits.RotateLeft32(x6+x5, 9)
		x4 ^= bits.RotateLeft32(x7+x6, 13)
		x5 ^= bits.RotateLeft32(x4+x7, 18)

		x11 ^= bits.RotateLeft32(x10+x9, 7)
		x8 ^= bits.RotateLeft32(x11+x10, 9)
		x9 ^= bits.RotateLeft32(x8+x11, 13)
		x10 ^= bits.RotateLeft32(x9+x8, 18)

		x12 ^= bits.RotateLeft32(x15+x14, 7)
		x13 ^= bits.RotateLeft32(x12+x15, 9)
		x14 ^= bits.RotateLeft32(x13+x12, 13)
		x15 ^= bits.RotateLeft32(x14+x13, 18)
	}
	x0 += w0
	x1 += w1
	x2 += w2
	x3 += w3
	x4 += w4
	x5 += w5
	x6 += w6
	x7 += w7
	x8 += w8
	x9 += w9
	x10 += w10
	x11 += w11
	x12 += w12
	x13 += w13
	x14 += w14
	x15 += w15

	out[0], tmp[0] = x0, x0
	out[1], tmp[1] = x1, x1
	out[2], tmp[2] = x2, x2
	out[3], tmp[3] = x3, x3
	out[4], tmp[4] = x4, x4
	out[5], tmp[5] = x5, x5
	out[6], tmp[6] = x6, x6
	out[7], tmp[7] = x7, x7
	out[8], tmp[8] = x8, x8
	out[9], tmp[9] = x9, x9
	out[10], tmp[10] = x10, x10
	out[11], tmp[11] = x11, x11
	out[12], tmp[12] = x12, x12
	out[13], tmp[13] = x13, x13
	out[14], tmp[14] = x14, x14
	out[15], tmp[15] = x15, x15
}

func blockMix(tmp *[16]uint32, inp, out []uint32, r int) {
	copy(tmp[:], inp[(2*r-1)*16:])
	for i := 0; i < 2*r; i += 2 {
		salsaXOR(tmp, inp[i*16:], out[i*8:])
		salsaXOR(tmp, inp[i*16+16:], out[i*8+r*16:])
	}
}

func smix(b []byte, r, N int, v, xy []uint32) {
	var tmp [16]uint32
	R := 32 * r
	x := xy
	y := xy[R:]

	j := 0
	for i := 0; i < R; i++ {
		x[i] = binary.LittleEndian.Uint32(b[j:])
		j += 4
	}
	for i := 0; i < N; i += 2 {
		copy(v[i*R:], x[:R])
		blockMix(&tmp, x, y, r)
		copy(v[(i+1)*R:], y[:R])
		blockMix(&tmp, y, x, r)
	}
	for i := 0; i < N; i += 2 {
		j := int(x[R-16] & uint32(N-1))
		for k := 0; k < R; k++ {
			x[k] ^= v[j*R+k]
		}
		blockMix(&tmp, x, y, r)

		j = int(y[R-16] & uint32(N-1))
		for k := 0; k < R; k++ {
			y[k] ^= v[j*R+k]
		}
		blockMix(&tmp, y, x, r)
	}
	j = 0
	for _, xi := range x[:R] {
		binary.LittleEndian.PutUint32(b[j:], xi)
		j += 4
	}
}

func scryptKey(password, salt []byte, N, r, p, keyLen int) ([]byte, error) {
	if N <= 1 || N&(N-1) != 0 {
		return nil, errors.New("scrypt: N must be > 1 and a power of 2")
	}
	if r <= 0 || p <= 0 || keyLen <= 0 {
		return nil, errors.New("scrypt: invalid parameters")
	}
	b := pbkdf2SHA256(password, salt, 1, p*128*r)
	xy := make([]uint32, 64*r)
	v := make([]uint32, 32*N*r)
	for i := 0; i < p; i++ {
		smix(b[i*128*r:], r, N, v, xy)
	}
	return pbkdf2SHA256(password, b, 1, keyLen), nil
}
