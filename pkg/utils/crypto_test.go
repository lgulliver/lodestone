package utils

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateAndValidateJWT(t *testing.T) {
	userID := uuid.New()
	secret := "test-secret"

	token, err := GenerateJWT(userID, secret, time.Hour)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	got, err := ValidateJWT(token, secret)
	require.NoError(t, err)
	assert.Equal(t, userID, got)
}

func TestValidateJWTWrongSecret(t *testing.T) {
	token, err := GenerateJWT(uuid.New(), "right", time.Hour)
	require.NoError(t, err)
	_, err = ValidateJWT(token, "wrong")
	assert.Error(t, err)
}

func TestValidateJWTExpired(t *testing.T) {
	token, err := GenerateJWT(uuid.New(), "s", -time.Hour)
	require.NoError(t, err)
	_, err = ValidateJWT(token, "s")
	assert.Error(t, err)
}

func TestValidateJWTMalformed(t *testing.T) {
	_, err := ValidateJWT("not.a.jwt", "s")
	assert.Error(t, err)
}

func TestComputeSHA256(t *testing.T) {
	// Known vector for empty string.
	assert.Equal(t, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", ComputeSHA256([]byte("")))
}

func TestComputeSHA1(t *testing.T) {
	assert.Equal(t, "da39a3ee5e6b4b0d3255bfef95601890afd80709", ComputeSHA1([]byte("")))
}

func TestComputeSHA256FromReader(t *testing.T) {
	got, err := ComputeSHA256FromReader(strings.NewReader(""))
	require.NoError(t, err)
	assert.Equal(t, ComputeSHA256([]byte("")), got)

	got, err = ComputeSHA256FromReader(strings.NewReader("hello"))
	require.NoError(t, err)
	assert.Equal(t, ComputeSHA256([]byte("hello")), got)
}

func TestComputeSHA1FromReader(t *testing.T) {
	got, err := ComputeSHA1FromReader(strings.NewReader("hello"))
	require.NoError(t, err)
	assert.Equal(t, ComputeSHA1([]byte("hello")), got)
}

func TestDecodeBase64(t *testing.T) {
	got, err := DecodeBase64("aGVsbG8=")
	require.NoError(t, err)
	assert.Equal(t, []byte("hello"), got)

	_, err = DecodeBase64("!!!not base64!!!")
	assert.Error(t, err)
}
