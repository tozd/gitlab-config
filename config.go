package config

type Configuration struct {
	Project map[string]interface{}
	Avatar  string `json:",omitempty" yaml:",omitempty"`
}
