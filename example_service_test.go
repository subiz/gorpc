package gorpc

import (
	"errors"
)

type ExampleDispatcherService struct {
	state int
}

func (s *ExampleDispatcherService) Get() int { return s.state }

func (s *ExampleDispatcherService) Set(x int) { s.state = x }

func (s *ExampleDispatcherService) GetError42() (int, error) {
	if s.state == 42 {
		return 0, errors.New("error42")
	}
	return s.state, nil
}

func (s *ExampleDispatcherService) privateFunc(string) { s.state = 0 }
