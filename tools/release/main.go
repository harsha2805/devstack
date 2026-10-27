// Command release signs devstack's update feed with the offline release key
// (see docs/RELEASING.md).
//
//	go run ./tools/release keygen            create the key (once) and print the public key to pin
//	go run ./tools/release check             confirm the key matches one in update.TrustedKeys
//	go run ./tools/release sign latest.json  add "signature" and verify it as a released app will
//
// The private key never belongs in the repository. Default location:
// %APPDATA%\devstack-release\update-signing-key.pem (override: DEVSTACK_UPDATE_KEY).
package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/harsha2805/devstack/internal/update"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	keyPath, err := keyFile()
	if err != nil {
		return err
	}
	switch {
	case len(args) == 1 && args[0] == "keygen":
		if _, err := os.Stat(keyPath); err == nil {
			return fmt.Errorf("a key already exists at %s; refusing to overwrite it", keyPath)
		}
		pub, priv, err := ed25519.GenerateKey(nil)
		if err != nil {
			return err
		}
		der, err := x509.MarshalPKCS8PrivateKey(priv)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
			return err
		}
		fmt.Printf("Private key written to %s. Back it up somewhere safe and offline.\n", keyPath)
		fmt.Println("Public key for update.TrustedKeys:")
		fmt.Println(base64.StdEncoding.EncodeToString(pub))
		return nil

	case len(args) == 1 && args[0] == "check":
		priv, err := loadKey(keyPath)
		if err != nil {
			return err
		}
		pub := base64.StdEncoding.EncodeToString(priv.Public().(ed25519.PublicKey))
		if !slices.Contains(update.TrustedKeys, pub) {
			return fmt.Errorf("the key at %s is not in update.TrustedKeys", keyPath)
		}
		fmt.Println("Signing key matches a trusted app key.")
		return nil

	case len(args) == 2 && args[0] == "sign":
		priv, err := loadKey(keyPath)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(args[1])
		if err != nil {
			return err
		}
		var i update.Info
		dec := json.NewDecoder(bytes.NewReader(bytes.TrimPrefix(b, []byte("\xef\xbb\xbf"))))
		if err := dec.Decode(&i); err != nil {
			return fmt.Errorf("%s: %w", args[1], err)
		}
		out, err := update.Sign(i, priv)
		if err != nil {
			return err
		}
		// Verify exactly as a released app would before the file is published.
		if _, err := update.Parse(out, update.TrustedKeys); err != nil {
			return fmt.Errorf("signed feed doesn't verify: %w", err)
		}
		if err := os.WriteFile(args[1], out, 0o644); err != nil {
			return err
		}
		fmt.Printf("Signed feed for %s: %s\n", i.Version, args[1])
		return nil
	}
	return errors.New("usage: release keygen | check | sign <latest.json>")
}

func keyFile() (string, error) {
	if p := os.Getenv("DEVSTACK_UPDATE_KEY"); p != "" {
		return p, nil
	}
	dir, err := os.UserConfigDir() // %APPDATA%
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "devstack-release", "update-signing-key.pem"), nil
}

func loadKey(path string) (ed25519.PrivateKey, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("no signing key at %s; restore it from backup (or set DEVSTACK_UPDATE_KEY)", path)
	}
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, fmt.Errorf("%s is not a PEM key", path)
	}
	k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	priv, ok := k.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("%s is not an Ed25519 key", path)
	}
	return priv, nil
}
