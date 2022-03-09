// This program is used to populate the passwd and passwd2 fields in UltraVNC's ultravnc.ini file.
// UltraVNC's password encryption algorithm is based on DES, but Ansible's password_hash function
// does not support DES encryption. This program fills that gap.
//
// UltraVNC encrypts passwords in the following way:
//
//   1. It clips passwords to a maximum length of 8 characters, or right-pads them with null
//      characters (i.e. 0x00) if the password length is less than 8.
//   2. It encrypts the clipped or padded password using DES with a known key.
//   3. It appends a seemingly arbitrary byte at the end of the resulting 8-byte hash.
//   4. It hex-encodes the resulting 9 bytes and stores it in the ultravnc.ini file.
//
// This program takes a password as its only command-line argument, encrypts it as per the steps
// above, and prints the resulting hex-encoded hash to stdout.
//
// See https://ivanderevianko.com/2012/08/ultravnc-password-encription-and-decription-c-en for more
// details on how UltraVNC stores passwords.
//
// Note: UltraVNC provides two password-related command-line utilities: setpasswd.exe and
// createpassword.exe. Netiher seems to change ultravnc.ini.

package main

import (
	"crypto/des"
	"encoding/hex"
	"fmt"
	"os"
)

// Taken from https://ivanderevianko.com/2012/08/ultravnc-password-encription-and-decription-c-en.
var ultraVNCDESKey = []byte{0xE8, 0x4A, 0xD6, 0x60, 0xC4, 0x72, 0x1A, 0xE0}

// encrypt encrypts a password using UltraVNC's algorithm.
func encrypt(password string) string {
	// Pad the password with zeroes, then take the first 8 bytes.
	password = password + "\x00\x00\x00\x00\x00\x00\x00\x00"
	password = password[:8]

	// Create a DES cipher using the same key as UltraVNC.
	block, err := des.NewCipher(ultraVNCDESKey)
	if err != nil {
		panic(err)
	}

	// Encrypt password.
	encryptedPassword := make([]byte, block.BlockSize())
	block.Encrypt(encryptedPassword, []byte(password))

	// Append an arbitrary byte as per UltraVNC's algorithm.
	encryptedPassword = append(encryptedPassword, 0)

	// Return encrypted password as a hex-encoded string.
	return hex.EncodeToString(encryptedPassword)
}

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("Usage: %s <password>\n", os.Args[0])
		os.Exit(1)
	}
	password := os.Args[1]
	fmt.Println(encrypt(password))
}
