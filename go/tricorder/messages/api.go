package messages

import (
	"github.com/Symantec/tricorder/go/tricorder/types"
)

type Range struct {
	Lower *float64 `json:"lower,omitempty"`
	Upper *float64 `json:"upper,omitempty"`
	Count uint64   `json:"count"`
}

type Distribution struct {
	Min    float64  `json:"min"`
	Max    float64  `json:"max"`
	Avg    float64  `json:"avg"`
	Median float64  `json:"median"`
	Count  uint64   `json:"count"`
	Ranges []*Range `json:"ranges,omitempty"`
}

type Value struct {
	Kind              types.Type    `json:"kind"`
	IntValue          *int64        `json:"intValue,omitempty"`
	UintValue         *uint64       `json:"uintValue,omitempty"`
	FloatValue        *float64      `json:"floatValue,omitempty"`
	StringValue       *string       `json:"stringValue,omitempty"`
	DistributionValue *Distribution `json:"distributionValue,omitempty"`
}

type PathResponse struct {
	Path        string `json:"path"`
	Description string `json:"description"`
	Unit        string `json:"unit"`
	Value       *Value `json:"value"`
}

type JsonPathResponse struct {
	*PathResponse
	Url string `json:"url"`
}

type ListRequest struct {
	AbsPath string
}

type ListResponse struct {
	Items []*PathResponse
}

type JsonListResponse struct {
	Items []*JsonPathResponse
}
