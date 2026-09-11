package hash

import (
	"encoding/json"
	"os"
)

func LoadStore(path string) Store {
	s := Store{
		Expected: map[string]string{},
		Results:  map[string]*Result{},
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s
		}
		panic(err)
	}
	if err := json.Unmarshal(data, &s); err != nil {
		panic(err)
	}
	if s.Expected == nil {
		s.Expected = map[string]string{}
	}
	if s.Results == nil {
		s.Results = map[string]*Result{}
	}
	for name, rec := range s.Results {
		if rec != nil && rec.Status == StatusRunning {
			delete(s.Results, name)
		}
	}
	return s
}

func SaveStore(path string, s Store) {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		panic(err)
	}
}
