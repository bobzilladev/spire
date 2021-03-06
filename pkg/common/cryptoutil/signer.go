package cryptoutil

import (
	"context"
	"crypto"
	"crypto/x509"
	"errors"
	"fmt"
	"io"

	"github.com/spiffe/spire/proto/server/keymanager"
)

type KeyManagerSigner struct {
	km        keymanager.KeyManager
	keyId     string
	publicKey crypto.PublicKey
}

var _ crypto.Signer = (*KeyManagerSigner)(nil)

func NewKeyManagerSigner(km keymanager.KeyManager, keyId string, publicKey crypto.PublicKey) *KeyManagerSigner {
	return &KeyManagerSigner{
		km:        km,
		keyId:     keyId,
		publicKey: publicKey,
	}
}

func (s *KeyManagerSigner) Public() crypto.PublicKey {
	return s.publicKey
}

func (s *KeyManagerSigner) SignContext(ctx context.Context, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	resp, err := s.km.SignData(ctx, &keymanager.SignDataRequest{
		KeyId:         s.keyId,
		Data:          digest,
		HashAlgorithm: keymanager.HashAlgorithm(opts.HashFunc()),
	})
	if err != nil {
		return nil, err
	}
	if len(resp.Signature) == 0 {
		return nil, fmt.Errorf("response missing signature data")
	}
	return resp.Signature, nil
}

func (s *KeyManagerSigner) Sign(_ io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	// rand is purposefully ignored since it can't be communicated between
	// the plugin boundary. The crypto.Signer interface implies this is ok
	// when it says "possibly using entropy from rand".
	return s.SignContext(context.Background(), digest, opts)
}

func GenerateKeyRaw(ctx context.Context, km keymanager.KeyManager, keyId string, algorithm keymanager.KeyAlgorithm) ([]byte, error) {
	resp, err := km.GenerateKey(ctx, &keymanager.GenerateKeyRequest{
		KeyId:        keyId,
		KeyAlgorithm: algorithm,
	})
	if err != nil {
		return nil, err
	}
	if resp.PublicKey == nil {
		return nil, errors.New("response missing public key")
	}
	return resp.PublicKey.PkixData, nil
}

func GenerateKey(ctx context.Context, km keymanager.KeyManager, keyId string, algorithm keymanager.KeyAlgorithm) (crypto.PublicKey, error) {
	pkixData, err := GenerateKeyRaw(ctx, km, keyId, algorithm)
	if err != nil {
		return nil, err
	}
	publicKey, err := x509.ParsePKIXPublicKey(pkixData)
	if err != nil {
		return nil, fmt.Errorf("unable to parse public key pkix data: %v", err)
	}
	return publicKey, nil
}

func GenerateKeyAndSigner(ctx context.Context, km keymanager.KeyManager, keyId string, algorithm keymanager.KeyAlgorithm) (*KeyManagerSigner, error) {
	publicKey, err := GenerateKey(ctx, km, keyId, algorithm)
	if err != nil {
		return nil, err
	}
	return NewKeyManagerSigner(km, keyId, publicKey), nil
}
