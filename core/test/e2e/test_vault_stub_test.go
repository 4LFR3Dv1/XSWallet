package e2e

import "bytes"

// e2eTestVault is a lightweight preimage crypto stub for swap.Engine tests.
type e2eTestVault struct{}

func (e2eTestVault) EncryptPreimage(b []byte) ([]byte, error) {
	return append([]byte("enc:"), b...), nil
}

func (e2eTestVault) DecryptPreimage(b []byte) ([]byte, error) {
	return bytes.TrimPrefix(b, []byte("enc:")), nil
}
