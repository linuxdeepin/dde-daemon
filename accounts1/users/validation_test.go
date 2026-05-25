// SPDX-FileCopyrightText: 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

package users

import (
	"strings"
	"testing"
)

func TestIsValidUsername(t *testing.T) {
	maxLen := LoginNameMaxSize()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"lowercase", "root", false},
		{"uppercase", "ADMIN", false},
		{"mixed case", "JohnDoe", false},
		{"with digits", "user123", false},
		{"digit first then letter", "0xroot", false},
		{"underscore in middle", "my_user", false},
		{"underscore first", "_start", false},
		{"single underscore", "_", false},
		{"dot in middle", "user.name", false},
		{"dot first", ".hidden", false},
		{"dash in middle", "my-user", false},
		{"dash at end", "user-", false},
		{"trailing dollar sign", "computer$", false},
		{"trailing dollar with dash", "my-user$", false},
		{"single letter", "a", false},
		{"max length valid", strings.Repeat("a", maxLen), false},
		{"all valid chars", "aB3_.-z", false},
		{"all valid chars trailing dollar", "aB3_.-z$", false},
		{"dot dot word", "..foo", false},
		{"digit then trailing dollar", "1$", false},
		{"underscore then trailing dollar", "_$", false},
		{"max length minus one plus dollar", strings.Repeat("a", maxLen-1) + "$", false},

		{"empty string", "", true},
		{"single dot", ".", true},
		{"double dot", "..", true},

		{"dash first", "-user", true},
		{"single dash", "-", true},
		{"all dashes", "---", true},

		{"all digits", "12345", true},
		{"single digit zero", "0", true},
		{"single digit nine", "9", true},

		{"dollar in middle", "u$er", true},
		{"dollar at start", "$user", true},
		{"multiple dollars", "u$e$r", true},
		{"isolated dollar", "$", true},
		{"exceeds max length with trailing dollar", strings.Repeat("a", maxLen) + "$", true},
		{"max length with trailing dollar to match maxLen", strings.Repeat("a", maxLen-1) + "$", false},

		{"contains space", "user name", true},
		{"contains colon", "user:name", true},
		{"contains at sign", "user@host", true},
		{"contains newline", "user\nname", true},
		{"contains carriage return", "user\rname", true},
		{"contains tab", "user\tname", true},
		{"contains slash", "user/name", true},
		{"contains backslash", "user\\name", true},
		{"contains semicolon", "user;name", true},
		{"contains hash", "user#name", true},
		{"contains exclamation", "user!", true},
		{"contains asterisk", "user*", true},
		{"contains null byte", "user\x00name", true},
		{"contains DEL", "user\x7fname", true},
		{"non-ASCII UTF-8", "usér", true},

		{"exceeds max length", strings.Repeat("a", maxLen+1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := isValidUsername(tt.input)
			if (err != nil) != tt.wantErr {
				if tt.wantErr {
					t.Errorf("isValidUsername(%q) = nil; want error", tt.input)
				} else {
					t.Errorf("isValidUsername(%q) = %v; want nil", tt.input, err)
				}
			}
		})
	}
}

func TestIsValidCryptHash(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		// Valid: descrypt — [./0-9A-Za-z]{13}
		{"descrypt", "rEK1ecacwY7Ec", false},

		// Valid: bsdicrypt — _[./0-9A-Za-z]{19}
		{"bsdicrypt", "_J9..CCCCXBrJUQKYwfM", false},

		// Valid: md5crypt — $1$[^$:\n]{1,8}$[./0-9A-Za-z]{22}
		{"md5crypt", "$1$5heVhQ1S$6Jv5CZTPb5bEidVHKMLYQ0", false},

		// Valid: sha256crypt — $5$salt$hash
		{"sha256crypt", "$5$rounds=5000$saltsalt$Qd1q7XbC7pFRCXbmJ4zBvqJK9yB0KV.YHqyLHtQ8Hj5", false},

		// Valid: sha512crypt — $6$salt$hash
		{"sha512crypt", "$6$rounds=5000$saltsalt$WV2zoZJ0V2rFPt.mqU0bJ5NqOq0Pw/T6b62Cn9LdWmGdQhv5KjI6q0Q1YqW3Yd0F9fZEPNi3s3xOt4y1wX5K.", false},

		// Valid: bcrypt — $2b$10$salthash
		{"bcrypt", "$2b$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy", false},

		// Valid: bcrypt variant $2a$
		{"bcrypt 2a", "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy", false},

		// Valid: bcrypt variant $2y$
		{"bcrypt 2y", "$2y$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy", false},

		// Valid: yescrypt — $y$...
		{"yescrypt", "$y$j9T$salt$wu3F0f0Y.0xRfZb7YUvXRMFOXJymrV3NWGKvhjqYUBC", false},

		// Valid: gost-yescrypt — $gy$...
		{"gost-yescrypt", "$gy$j9T$salt$wu3F0f0Y.0xRfZb7YUvXRMFOXJymrV3NWGKvhjqYUBC", false},

		// Valid: scrypt — $7$...
		{"scrypt", "$7$C6..../....hF6U8VWl2Sg5OHa1/$s2MLYiJZyW9b8g1k01cz3yBDEsSja8ruRhZJvM0D7t4", false},

		// Valid: sha1crypt — $sha1$...
		{"sha1crypt", "$sha1$12345$FBELCwsB$M6FRylBqD9nKgSXSbLIs2MsjD3NmC", false},

		// Valid: SunMD5 — $md5...
		{"sunmd5", "$md5,rounds=12345$UBUqeiUQ$mQVVoVfN03ZMcK3gRl0kS1", false},

		// Valid: sm3crypt — $sm3$...
		{"sm3crypt", "$sm3$rounds=5000$saltsalt$WV2zoZJ0V2rFPt.mqU0bJ5NqOq0Pw/T6b62Cn9LdWmGdQhv5KjI6q0Q1YqW3Yd0F9fZEPNi3s3xOt4y1wX5K.", false},

		// Valid: sm3-yescrypt — $sm3y$...
		{"sm3yescrypt", "$sm3y$j9T$salt$wu3F0f0Y.0xRfZb7YUvXRMFOXJymrV3NWGKvhjqYUBC", false},

		// Valid: NT — $3$$hex
		{"nt", "$3$$8846f7eaee8fb117ad96bdd264e92957", false},

		// Valid: edge cases with allowed chars
		{"single dollar", "$", false},
		{"dollar dot slash", "$./", false},
		{"only base64 crypt chars", "./0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz", false},
		{"single printable char", "a", false},
		{"underscore prefix bsdicrypt style", "_validHashChars123", false},

		// Invalid: empty
		{"empty", "", true},

		// Invalid: forbidden chars per crypt(5) — whitespace
		{"contains space", "$6$salt$hash with space", true},
		{"contains tab", "$6$salt$hash\ttab", true},
		{"contains newline", "$6$salt$hash\nnewline", true},
		{"contains carriage return", "$6$salt$hash\rcr", true},

		// Invalid: forbidden chars per crypt(5) — delimiters
		{"contains colon", "$6$salt$hash:colon", true},
		{"contains semicolon", "$6$salt$hash;semicolon", true},
		{"contains asterisk", "$6$salt$hash*asterisk", true},
		{"contains exclamation", "$6$salt$hash!bang", true},
		{"contains backslash", "$6$salt$hash\\backslash", true},

		// Invalid: non-printable ASCII
		{"contains null byte", "$6$salt$\x00hash", true},
		{"contains DEL", "$6$salt$\x7fhash", true},
		{"contains control char 0x01", "$6$salt$\x01hash", true},
		{"contains control char 0x1f", "$6$salt$\x1fhash", true},

		// Invalid: non-ASCII
		{"non-ASCII UTF-8", "$6$salt$häsH", true},
		{"high byte 0x80", "$6$salt$\x80hash", true},
		{"high byte 0xff", "$6$salt$\xffhash", true},

		// Invalid: forbidden char at start
		{"colon at start", ":hash", true},
		{"semicolon at start", ";hash", true},
		{"asterisk at start", "*hash", true},
		{"exclamation at start", "!hash", true},
		{"backslash at start", "\\hash", true},
		{"space at start", " hash", true},

		// Invalid: forbidden char at end
		{"colon at end", "$6$salt$hash:", true},
		{"semicolon at end", "$6$salt$hash;", true},
		{"asterisk at end", "$6$salt$hash*", true},
		{"exclamation at end", "$6$salt$hash!", true},
		{"backslash at end", "$6$salt$hash\\", true},
		{"space at end", "$6$salt$hash ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := isValidCryptHash(tt.input)
			if (err != nil) != tt.wantErr {
				if tt.wantErr {
					t.Errorf("isValidCryptHash(%q) = nil; want error", tt.input)
				} else {
					t.Errorf("isValidCryptHash(%q) = %v; want nil", tt.input, err)
				}
			}
		})
	}
}

func FuzzIsValidCryptHash(f *testing.F) {
	f.Add("$6$rounds=5000$saltsalt$Qd1q7XbC7pFRCXbmJ4zBvqJK9yB0KV.YHqyLHtQ8Hj5")
	f.Add("$1$5heVhQ1S$6Jv5CZTPb5bEidVHKMLYQ0")
	f.Add("$2b$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy")
	f.Add("rEK1ecacwY7Ec")
	f.Add("_J9..CCCCXBrJUQKYwfM")
	f.Add("$3$$8846f7eaee8fb117ad96bdd264e92957")
	f.Add("")
	f.Add("hash with space")
	f.Add("hash:colon")
	f.Add("hash;semi")
	f.Add("hash*star")
	f.Add("hash!bang")
	f.Add("hash\\back")
	f.Add("hash\x00null")
	f.Add("häsH")

	f.Fuzz(func(t *testing.T, hash string) {
		err := isValidCryptHash(hash)
		if err != nil {
			return
		}

		if hash == "" {
			t.Errorf("accepted empty hash")
		}

		forbidden := [6]byte{' ', ':', ';', '*', '!', '\\'}
		for i := 0; i < len(hash); i++ {
			b := hash[i]

			if b < 32 || b > 126 {
				t.Errorf("accepted non-printable byte 0x%02x at pos %d", b, i)
			}

			for _, f := range forbidden {
				if b == f {
					t.Errorf("accepted forbidden byte 0x%02x (%c) at pos %d", b, b, i)
				}
			}
		}
	})
}

func FuzzIsValidUsername(f *testing.F) {
	f.Add("root")
	f.Add("user-name")
	f.Add(".")
	f.Add("-bad")
	f.Add("123")
	f.Add("a$b")
	f.Add("computer$")
	f.Add("")
	f.Add("..")
	f.Add("_underscore")
	f.Add("0xstart")
	f.Add("a")
	f.Add("1$")
	f.Add("_$")
	f.Add("$")

	f.Fuzz(func(t *testing.T, name string) {
		err := isValidUsername(name)
		if err != nil {
			return
		}

		if name == "" || name == "." || name == ".." {
			t.Fatalf("accepted forbidden name %q", name)
		}

		if len(name) > LoginNameMaxSize() {
			t.Fatalf("accepted name exceeding max length (%d bytes)", len(name))
		}

		if len(name) > 0 && name[0] == '-' {
			t.Fatalf("accepted dash-first name %q", name)
		}

		allDigit := true
		for i := 0; i < len(name); i++ {
			if name[i] < '0' || name[i] > '9' {
				allDigit = false
				break
			}
		}

		if allDigit && len(name) > 0 {
			t.Fatalf("accepted all-numeric name %q", name)
		}

		for i := 0; i < len(name); i++ {
			b := name[i]
			valid := (b >= 'a' && b <= 'z') ||
				(b >= 'A' && b <= 'Z') ||
				(b >= '0' && b <= '9') ||
				b == '_' || b == '.' || b == '-'

			if b == '$' && i == len(name)-1 {
				valid = true
			}

			if !valid {
				t.Fatalf("accepted invalid byte 0x%02x at pos %d in %q", b, i, name)
			}
		}

		if len(name) > 0 {
			b := name[0]
			firstValid := (b >= 'a' && b <= 'z') ||
				(b >= 'A' && b <= 'Z') ||
				(b >= '0' && b <= '9') ||
				b == '_' || b == '.'

			if !firstValid {
				t.Fatalf("accepted invalid first byte 0x%02x in %q", b, name)
			}
		}
	})
}
