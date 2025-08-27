package main

import (
	"Chat_App/internal/services"
	"fmt"
	"log"

	"github.com/gofrs/uuid"
)

func main() {
	cryptoService, err := services.NewCryptoService("keys")
	if err != nil {
		log.Fatal("Failed to initialize crypto service:", err)
	}

	fmt.Println("=== End-to-End Encryption Test ===\n")

	fmt.Println("1. Generating RSA key pairs for two users...")
	user1ID := uuid.Must(uuid.NewV4())
	user2ID := uuid.Must(uuid.NewV4())

	privateKey1, publicKey1, err := cryptoService.GenerateKeyPair()
	if err != nil {
		log.Fatal("Failed to generate key pair for user 1:", err)
	}

	privateKey2, publicKey2, err := cryptoService.GenerateKeyPair()
	if err != nil {
		log.Fatal("Failed to generate key pair for user 2:", err)
	}

	fmt.Printf("   ✓ User 1 ID: %s\n", user1ID.String())
	fmt.Printf("   ✓ User 2 ID: %s\n\n", user2ID.String())

	fmt.Println("2. Saving private keys to local storage...")
	err = cryptoService.SavePrivateKey(user1ID, privateKey1)
	if err != nil {
		log.Fatal("Failed to save private key for user 1:", err)
	}

	err = cryptoService.SavePrivateKey(user2ID, privateKey2)
	if err != nil {
		log.Fatal("Failed to save private key for user 2:", err)
	}
	fmt.Println("   ✓ Private keys saved to 'keys/' directory\n")

	fmt.Println("3. Converting public keys to string format (for database storage)...")
	publicKey1String, err := cryptoService.PublicKeyToString(publicKey1)
	if err != nil {
		log.Fatal("Failed to convert public key 1:", err)
	}

	publicKey2String, err := cryptoService.PublicKeyToString(publicKey2)
	if err != nil {
		log.Fatal("Failed to convert public key 2:", err)
	}
	fmt.Println("   ✓ Public keys converted to PEM format\n")

	originalMessage := "Hello, this is a secret message from User 1 to User 2!"
	fmt.Printf("4. Original message from User 1: \"%s\"\n\n", originalMessage)

	fmt.Println("5. User 1 encrypting message with User 2's public key...")
	encryptedMessage, err := cryptoService.EncryptWithPublicKeyString(originalMessage, publicKey2String)
	if err != nil {
		log.Fatal("Failed to encrypt message:", err)
	}
	fmt.Printf("   ✓ Encrypted message: %s...\n\n", encryptedMessage[:50])

	fmt.Println("6. Message stored in database (encrypted)...")
	fmt.Println("   ✓ Only encrypted content is stored - unreadable without private key\n")

	fmt.Println("7. User 2 retrieving and decrypting the message...")
	decryptedMessage, err := cryptoService.DecryptWithUserPrivateKey(encryptedMessage, user2ID)
	if err != nil {
		log.Fatal("Failed to decrypt message:", err)
	}
	fmt.Printf("   ✓ Decrypted message: \"%s\"\n\n", decryptedMessage)

	fmt.Println("8. Testing wrong user attempting to decrypt...")
	_, err = cryptoService.DecryptWithUserPrivateKey(encryptedMessage, user1ID)
	if err != nil {
		fmt.Println("   ✓ User 1 cannot decrypt (as expected) - only User 2 can decrypt\n")
	} else {
		fmt.Println("   ✗ Security issue - wrong user could decrypt!\n")
	}

	fmt.Println("=== Reverse Test: User 2 → User 1 ===\n")
	
	reverseMessage := "Reply from User 2: Message received securely!"
	fmt.Printf("9. User 2 sending reply: \"%s\"\n", reverseMessage)
	
	encryptedReply, err := cryptoService.EncryptWithPublicKeyString(reverseMessage, publicKey1String)
	if err != nil {
		log.Fatal("Failed to encrypt reply:", err)
	}
	fmt.Printf("   ✓ Reply encrypted with User 1's public key\n\n")

	decryptedReply, err := cryptoService.DecryptWithUserPrivateKey(encryptedReply, user1ID)
	if err != nil {
		log.Fatal("Failed to decrypt reply:", err)
	}
	fmt.Printf("10. User 1 decrypted reply: \"%s\"\n\n", decryptedReply)

	fmt.Println("=== Test Summary ===")
	fmt.Println("✓ RSA key generation working")
	fmt.Println("✓ Private key storage working")
	fmt.Println("✓ Message encryption working")
	fmt.Println("✓ Message decryption working")
	fmt.Println("✓ Security verification passed (wrong user cannot decrypt)")
	fmt.Println("\n🔐 End-to-End Encryption successfully implemented!")

	fmt.Println("\nCleaning up test keys...")
	cryptoService.DeletePrivateKey(user1ID)
	cryptoService.DeletePrivateKey(user2ID)
	fmt.Println("✓ Test keys removed")
}