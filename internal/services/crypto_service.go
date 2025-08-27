package services

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gofrs/uuid"
)

type CryptoService struct {
	keysDir string
}

func NewCryptoService(keysDir string) (*CryptoService, error) {
	if err := os.MkdirAll(keysDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create keys directory: %w", err)
	}
	
	return &CryptoService{
		keysDir: keysDir,
	}, nil
}

func (s *CryptoService) GenerateKeyPair() (*rsa.PrivateKey, *rsa.PublicKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate private key: %w", err)
	}
	
	return privateKey, &privateKey.PublicKey, nil
}

func (s *CryptoService) SavePrivateKey(userID uuid.UUID, privateKey *rsa.PrivateKey) error {
	privateKeyPath := filepath.Join(s.keysDir, fmt.Sprintf("%s_private.pem", userID.String()))
	
	privateKeyFile, err := os.Create(privateKeyPath)
	if err != nil {
		return fmt.Errorf("failed to create private key file: %w", err)
	}
	defer privateKeyFile.Close()
	
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}
	
	if err := pem.Encode(privateKeyFile, privateKeyPEM); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}
	
	return nil
}

func (s *CryptoService) LoadPrivateKey(userID uuid.UUID) (*rsa.PrivateKey, error) {
	privateKeyPath := filepath.Join(s.keysDir, fmt.Sprintf("%s_private.pem", userID.String()))
	
	privateKeyData, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}
	
	block, _ := pem.Decode(privateKeyData)
	if block == nil {
		return nil, errors.New("failed to parse PEM block containing the private key")
	}
	
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}
	
	return privateKey, nil
}

func (s *CryptoService) PublicKeyToString(publicKey *rsa.PublicKey) (string, error) {
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", fmt.Errorf("failed to marshal public key: %w", err)
	}
	
	publicKeyPEM := &pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: publicKeyBytes,
	}
	
	publicKeyString := string(pem.EncodeToMemory(publicKeyPEM))
	return publicKeyString, nil
}

func (s *CryptoService) StringToPublicKey(publicKeyString string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(publicKeyString))
	if block == nil {
		return nil, errors.New("failed to parse PEM block containing the public key")
	}
	
	publicKeyInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}
	
	publicKey, ok := publicKeyInterface.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("not an RSA public key")
	}
	
	return publicKey, nil
}

func (s *CryptoService) EncryptMessage(message string, publicKey *rsa.PublicKey) (string, error) {
	messageBytes := []byte(message)
	
	label := []byte("")
	hash := sha256.New()
	
	ciphertext, err := rsa.EncryptOAEP(hash, rand.Reader, publicKey, messageBytes, label)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt message: %w", err)
	}
	
	encryptedMessage := base64.StdEncoding.EncodeToString(ciphertext)
	return encryptedMessage, nil
}

func (s *CryptoService) DecryptMessage(encryptedMessage string, privateKey *rsa.PrivateKey) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedMessage)
	if err != nil {
		return "", fmt.Errorf("failed to decode encrypted message: %w", err)
	}
	
	label := []byte("")
	hash := sha256.New()
	
	plaintext, err := rsa.DecryptOAEP(hash, rand.Reader, privateKey, ciphertext, label)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt message: %w", err)
	}
	
	return string(plaintext), nil
}

func (s *CryptoService) EncryptWithPublicKeyString(message string, publicKeyString string) (string, error) {
	publicKey, err := s.StringToPublicKey(publicKeyString)
	if err != nil {
		return "", err
	}
	
	return s.EncryptMessage(message, publicKey)
}

func (s *CryptoService) DecryptWithUserPrivateKey(encryptedMessage string, userID uuid.UUID) (string, error) {
	privateKey, err := s.LoadPrivateKey(userID)
	if err != nil {
		return "", err
	}
	
	return s.DecryptMessage(encryptedMessage, privateKey)
}
func (s *CryptoService) DeletePrivateKey(userID uuid.UUID) error {
	privateKeyPath := filepath.Join(s.keysDir, fmt.Sprintf("%s_private.pem", userID.String()))
	return os.Remove(privateKeyPath)
}
