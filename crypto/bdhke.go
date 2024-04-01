package crypto

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
)

const DomainSeparator = "Secp256k1_HashToCurve_Cashu_"

//     Generates a secp256k1 point from a message.

//     The point is generated by hashing the message with a domain separator and then
//     iteratively trying to compute a point from the hash. An increasing uint32 counter
//     (byte order little endian) is appended to the hash until a point is found that lies on the curve.

//     The chance of finding a valid point is 50% for every iteration. The maximum number of iterations
//     is 2**16. If no valid point is found after 2**16 iterations, a ValueError is raised (this should
//     never happen in practice).

// The domain separator is b"Secp256k1_HashToCurve_Cashu_" or
// bytes.fromhex("536563703235366b315f48617368546f43757276655f43617368755f").
func HashToCurve(message []byte) (*secp256k1.PublicKey, error) {
	msgToHash := sha256.Sum256(append([]byte(DomainSeparator), message...))
	var counter uint32 = 0
	for counter < uint32(math.Exp2(16)) {
		// little endian counter
		c := make([]byte, 4)
		binary.LittleEndian.PutUint32(c, counter)

		hash := sha256.Sum256(append(msgToHash[:], c...))
		pkHash := append([]byte{0x02}, hash[:]...)
		point, err := secp256k1.ParsePubKey(pkHash)
		if err != nil {
			counter++
			continue
		}
		if point.IsOnCurve() {
			return point, nil
		}
	}
	return nil, errors.New("No valid point found")
}

// B_ = Y + rG
func BlindMessage(secret string, r *secp256k1.PrivateKey) (*secp256k1.PublicKey,
	*secp256k1.PrivateKey, error) {

	var ypoint, rpoint, blindedMessage secp256k1.JacobianPoint
	Y, err := HashToCurve([]byte(secret))
	if err != nil {
		return nil, nil, err
	}
	Y.AsJacobian(&ypoint)

	rpub := r.PubKey()
	rpub.AsJacobian(&rpoint)

	// blindedMessage = Y + rG
	secp256k1.AddNonConst(&ypoint, &rpoint, &blindedMessage)
	blindedMessage.ToAffine()
	B_ := secp256k1.NewPublicKey(&blindedMessage.X, &blindedMessage.Y)

	return B_, r, nil
}

// C_ = kB_
func SignBlindedMessage(B_ *secp256k1.PublicKey, k *secp256k1.PrivateKey) *secp256k1.PublicKey {
	var bpoint, result secp256k1.JacobianPoint
	B_.AsJacobian(&bpoint)

	// result = k * B_
	secp256k1.ScalarMultNonConst(&k.Key, &bpoint, &result)
	result.ToAffine()
	C_ := secp256k1.NewPublicKey(&result.X, &result.Y)

	return C_
}

// C = C_ - rK
func UnblindSignature(C_ *secp256k1.PublicKey, r *secp256k1.PrivateKey,
	K *secp256k1.PublicKey) *secp256k1.PublicKey {

	var Kpoint, rKPoint, CPoint secp256k1.JacobianPoint
	K.AsJacobian(&Kpoint)

	var rNeg secp256k1.ModNScalar
	rNeg.NegateVal(&r.Key)

	secp256k1.ScalarMultNonConst(&rNeg, &Kpoint, &rKPoint)

	var C_Point secp256k1.JacobianPoint
	C_.AsJacobian(&C_Point)
	secp256k1.AddNonConst(&C_Point, &rKPoint, &CPoint)
	CPoint.ToAffine()

	C := secp256k1.NewPublicKey(&CPoint.X, &CPoint.Y)
	return C
}

// k * HashToCurve(secret) == C
func Verify(secret string, k *secp256k1.PrivateKey, C *secp256k1.PublicKey) bool {
	Y, err := HashToCurve([]byte(secret))
	if err != nil {
		return false
	}
	valid := verify(Y, k, C)
	if !valid {
		Y := HashToCurveDeprecated([]byte(secret))
		valid = verify(Y, k, C)
	}

	return valid
}

func verify(Y *secp256k1.PublicKey, k *secp256k1.PrivateKey, C *secp256k1.PublicKey) bool {
	var Ypoint, result secp256k1.JacobianPoint
	Y.AsJacobian(&Ypoint)

	secp256k1.ScalarMultNonConst(&k.Key, &Ypoint, &result)
	result.ToAffine()
	pk := secp256k1.NewPublicKey(&result.X, &result.Y)

	return C.IsEqual(pk)
}


// Deprecated HashToCurve 

func HashToCurveDeprecated(message []byte) *secp256k1.PublicKey {
	var point *secp256k1.PublicKey

	for point == nil || !point.IsOnCurve() {
		hash := sha256.Sum256(message)
		pkhash := append([]byte{0x02}, hash[:]...)
		point, _ = secp256k1.ParsePubKey(pkhash)
		message = hash[:]
	}
	return point
}

func BlindMessageDeprecated(secret string, r *secp256k1.PrivateKey) (*secp256k1.PublicKey, *secp256k1.PrivateKey) {
	var ypoint, rpoint, blindedMessage secp256k1.JacobianPoint

	Y := HashToCurveDeprecated([]byte(secret))
	Y.AsJacobian(&ypoint)

	rpub := r.PubKey()
	rpub.AsJacobian(&rpoint)

	// blindedMessage = Y + rG
	secp256k1.AddNonConst(&ypoint, &rpoint, &blindedMessage)
	blindedMessage.ToAffine()
	B_ := secp256k1.NewPublicKey(&blindedMessage.X, &blindedMessage.Y)

	return B_, r
}