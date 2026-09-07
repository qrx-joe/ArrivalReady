package worker

import "gopkg.in/yaml.v3"

func yamlUnmarshal(raw []byte, into *StandardFile) error {
	return yaml.Unmarshal(raw, into)
}
