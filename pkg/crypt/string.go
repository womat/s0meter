package crypt

// EncryptedString is a sym crypt string.
// It automatically gets encrypted when created.
// It also implements the Marshall/Unmarshall Text/Binary go interfaces for easier usage.
type EncryptedString struct {
	value string
}

// NewEncryptedString creates a new EncryptedString by encrypting the supplied plainTextValue.
func NewEncryptedString(plainTextValue string) *EncryptedString {
	s := EncryptedString{
		value: plainTextValue,
	}
	s.encrypt()
	return &s
}

// NewDecryptedString decrypts an encrypted string
func NewDecryptedString(encryptedValue string) string {
	s := EncryptedString{
		value: encryptedValue,
	}
	return s.Value()
}

func (v *EncryptedString) encrypt() {
	c := NewSymmetricEncryption()
	c.SetPlainText(v.value)
	v.value = c.GetCypherBase64()
}

func (v *EncryptedString) decrypt() string {
	c := NewSymmetricEncryption()
	c.SetCypherBase64(v.value)
	// ignore errors
	pt, _ := c.GetPlainText()
	return pt
}

// Value returns the decrypted plainTextValue.
func (v *EncryptedString) Value() string {
	return v.decrypt()
}

// Value returns the decrypted plainTextValue.
func (v *EncryptedString) String() string {
	return v.decrypt()
}

// EncryptedValue returns the encrypted value.
func (v *EncryptedString) EncryptedValue() string {
	return v.value
}

func (v *EncryptedString) MarshalText() ([]byte, error) {
	return []byte(v.value), nil
}

func (v *EncryptedString) UnmarshalText(text []byte) error {
	v.value = string(text)
	return nil
}

func (v *EncryptedString) MarshalBinary() ([]byte, error) {
	return []byte(v.value), nil
}

func (v *EncryptedString) UnmarshalBinary(text []byte) error {
	v.value = string(text)
	return nil
}
