package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDatabaseURL(t *testing.T) {
	d := &DatabaseConfig{Host: "h", Port: 5432, User: "u", Password: "p", DBName: "db", SSLMode: "disable"}
	assert.Equal(t, "host=h port=5432 user=u password=p dbname=db sslmode=disable", d.DatabaseURL())
}

func TestRedisAddr(t *testing.T) {
	r := &RedisConfig{Host: "localhost", Port: 6379}
	assert.Equal(t, "localhost:6379", r.RedisAddr())
}

func TestSetupLogging(t *testing.T) {
	// Exercises each level branch and both format branches; no panics expected.
	for _, lvl := range []string{"debug", "info", "warn", "error", "other"} {
		(&LoggingConfig{Level: lvl, Format: "json"}).SetupLogging()
	}
	(&LoggingConfig{Level: "info", Format: "console"}).SetupLogging()
	(&LoggingConfig{Level: "info", Format: "text"}).SetupLogging()
}

func TestInitLogger(t *testing.T) {
	InitLogger() // should not panic
}

func TestGetEnvDuration(t *testing.T) {
	t.Setenv("TEST_DUR", "2s")
	assert.Equal(t, 2*time.Second, getEnvDuration("TEST_DUR", time.Minute))

	t.Setenv("TEST_DUR_BAD", "notaduration")
	assert.Equal(t, time.Minute, getEnvDuration("TEST_DUR_BAD", time.Minute))

	assert.Equal(t, time.Hour, getEnvDuration("TEST_DUR_UNSET", time.Hour))
}

func TestGetEnvHelpers(t *testing.T) {
	t.Setenv("S", "val")
	assert.Equal(t, "val", getEnv("S", "def"))
	assert.Equal(t, "def", getEnv("UNSET_S", "def"))

	t.Setenv("I", "10")
	assert.Equal(t, 10, getEnvInt("I", 1))
	t.Setenv("I_BAD", "x")
	assert.Equal(t, 1, getEnvInt("I_BAD", 1))

	t.Setenv("I64", "100")
	assert.EqualValues(t, 100, getEnvInt64("I64", 1))
	t.Setenv("I64_BAD", "x")
	assert.EqualValues(t, 1, getEnvInt64("I64_BAD", 1))

	t.Setenv("B", "true")
	assert.True(t, getEnvBool("B", false))
	t.Setenv("B_BAD", "x")
	assert.False(t, getEnvBool("B_BAD", false))
}

func TestGetStorageEnv(t *testing.T) {
	assert.Equal(t, "def", getStorageEnv("P", "S", "T", "def"))

	t.Setenv("T3", "tertiary")
	assert.Equal(t, "tertiary", getStorageEnv("P3", "S3", "T3", "def"))

	t.Setenv("S2", "secondary")
	assert.Equal(t, "secondary", getStorageEnv("P2", "S2", "T2", "def"))

	t.Setenv("P1", "primary")
	assert.Equal(t, "primary", getStorageEnv("P1", "S1", "T1", "def"))
}
