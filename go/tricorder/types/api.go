package types

// Type represents the type of a metric value
type Type string

const (
	Int    Type = "int"
	Uint   Type = "uint"
	Float  Type = "float"
	String Type = "string"
	Dist   Type = "distribution"
	Time   Type = "time"
)
