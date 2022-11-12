package errors

import "encoding/json"

type MultiError struct {
	Errors []error
}

func (m *MultiError) Error() string {
	s := []string{}
	for _, err := range m.Errors {
		s = append(s, err.Error())
	}
	b, _ := json.Marshal(s)
	return string(b)
}
