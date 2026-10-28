package model

import "miwu-go/internal/miot/urn"

type LanguageMap map[string]string

type Specification struct {
	Description string    `json:"description"`
	Type        urn.URN   `json:"type"`
	Services    []Service `json:"services"`
}

type Service struct {
	IID         int         `json:"iid"`
	Description string      `json:"description"`
	Type        urn.URN     `json:"type"`
	Actions     *[]Action   `json:"actions"`
	Properties  *[]Property `json:"properties"`
	Translation string      `json:"translation"`
}

type Property struct {
	IID         int      `json:"iid"`
	Description string   `json:"description"`
	Type        urn.URN  `json:"type"`
	Format      string   `json:"format"`
	Access      []string `json:"access"`
	Source      *int     `json:"source"`
	Unit        *string  `json:"unit"`
	ValueList   *[]Value `json:"value-list"`
	ValueRange  *[]any   `json:"value-range"`
	Translation string   `json:"translation"`
}

type Action struct {
	IID         int     `json:"iid"`
	Description string  `json:"description"`
	Type        urn.URN `json:"type"`
	In          []any   `json:"in"`
	Out         []any   `json:"out"`
	Translation string  `json:"translation"`
}

type Value struct {
	Value       int    `json:"value"`
	Description string `json:"description"`
	Translation string `json:"translation"`
}
