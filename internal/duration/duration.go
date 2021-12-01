// Package duration defines the various durations used throughout
// Epinio, as timeouts, and other.
package duration

import (
	"time"

	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	deployment          = 10 * time.Minute
	serviceLoadBalancer = 5 * time.Minute
	podReady            = 5 * time.Minute

	// Fixed. __Not__ affected by the multiplier.
	pollInterval = 3 * time.Second
	userAbort    = 5 * time.Second
	logHistory   = 48 * time.Hour

	// Fixed. Standard number of attempts to retry various operations.
	RetryMax = 10
)

// Flags adds to viper flags
func Flags(pf *flag.FlagSet, argToEnv map[string]string) {
	pf.IntP("timeout-multiplier", "", 1, "Multiply timeouts by this factor")
	viper.BindPFlag("timeout-multiplier", pf.Lookup("timeout-multiplier"))
	argToEnv["timeout-multiplier"] = "EPINIO_TIMEOUT_MULTIPLIER"
}

// Multiplier returns the currently active timeout multiplier value
func Multiplier() time.Duration {
	return time.Duration(viper.GetInt("timeout-multiplier"))
}

// ToPodReady returns the duration to wait until giving up on getting
// a system domain
func ToPodReady() time.Duration {
	return Multiplier() * podReady
}

// ToDeployment returns the duration to wait for parts of a deployment
// to become ready
func ToDeployment() time.Duration {
	return Multiplier() * deployment
}

// ToServiceLoadBalancer
func ToServiceLoadBalancer() time.Duration {
	return Multiplier() * serviceLoadBalancer
}

//
// The following durations are not affected by the timeout multiplier.
//

// PollInterval returns the duration to use between polls of some kind
// of check.
func PollInterval() time.Duration {
	return pollInterval
}

// UserAbort returns the duration to wait when the user is given the
// chance to abort an operation
func UserAbort() time.Duration {
	return userAbort
}

// LogHistory returns the duration to reach into the past for tailing logs.
func LogHistory() time.Duration {
	return logHistory
}
