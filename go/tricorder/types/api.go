// Package types contains the various types for metric values.
package types

// Type represents the type of a metric value
type Type string

const (
	Bool     Type = "bool"
	Int      Type = "int"
	Uint     Type = "uint"
	Float    Type = "float"
	String   Type = "string"
	Dist     Type = "distribution"
	Time     Type = "time"
	Duration Type = "duration"

	// Used only in GoRPC
	GoTime Type = "goTime"
	// Used only in GoRPC
	GoDuration Type = "goDuration"
)
