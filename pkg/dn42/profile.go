package dn42

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

type Profile struct {
	data          map[string][]string
	originContent []byte
}

func (p *Profile) String() string {
	return string(p.originContent)
}

func (p *Profile) MarshalYAML() (interface{}, error) {
	stringsBuf := bytes.NewBufferString("")
	dec := yaml.NewEncoder(stringsBuf)
	dec.SetIndent(2)
	dec.Encode(p.Dump())
	contentB, err := io.ReadAll(stringsBuf)
	if err != nil {
		return nil, err
	}
	return string(contentB), nil
}

func (p *Profile) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.Dump())
}

func (p *Profile) Dump() map[string][]string {
	clone := make(map[string][]string)
	for k, v := range p.data {
		clone[k] = append([]string{}, v...)
	}
	return clone
}

func (p *Profile) GetFirst(key string) *string {
	if p != nil && p.data != nil {
		if vals, ok := p.data[key]; ok {
			if len(vals) > 0 {
				val := vals[0]
				return &val
			}
		}
	}
	return nil
}

func ParseProfile(path string) (*Profile, error) {
	result := new(Profile)
	result.data = make(map[string][]string)

	var err error = nil
	result.originContent, err = os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		pattern := regexp.MustCompile(`^([\w\d-_]+):\s*(.+)`)
		matches := pattern.FindStringSubmatch(line)
		if len(matches) >= 3 {
			group1 := matches[1]
			group2 := matches[2]

			if _, ok := result.data[group1]; !ok {
				result.data[group1] = make([]string, 0)
			}
			result.data[group1] = append(result.data[group1], group2)
		}
	}

	return result, nil
}
