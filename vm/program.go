package vm

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Program struct {
	Data       []byte      `json:"data"`
	Symbols    []Symbol    `json:"symbols"`
	Procedures []Procedure `json:"procedures"`
	Entry      string      `json:"entry"`
}

type Symbol struct {
	Name     string `json:"name"`
	Address  uint32 `json:"address"`
	Size     uint32 `json:"size"`
	Length   uint32 `json:"length"`
	ElemSize uint32 `json:"elem_size"`
	Decl     string `json:"decl"`
}

type Procedure struct {
	Name         string         `json:"name"`
	Address      uint32         `json:"address,omitempty"`
	Labels       map[string]int `json:"labels"`
	Instructions []Instruction  `json:"instructions"`
}

type Instruction struct {
	Op     string    `json:"op"`
	Args   []Operand `json:"args,omitempty"`
	Line   int       `json:"line,omitempty"`
	Source string    `json:"source,omitempty"`
}

type Operand struct {
	Kind   string `json:"kind"`
	Text   string `json:"text,omitempty"`
	Value  int64  `json:"value,omitempty"`
	Base   string `json:"base,omitempty"`
	Index  string `json:"index,omitempty"`
	Scale  int64  `json:"scale,omitempty"`
	Offset int64  `json:"offset,omitempty"`
	Size   int    `json:"size,omitempty"`
}

func ProgramFromJSON(data []byte) (*Program, error) {
	var program Program
	if err := json.Unmarshal(data, &program); err != nil {
		return nil, err
	}
	if err := program.Validate(); err != nil {
		return nil, err
	}
	return &program, nil
}

func MustProgramFromJSON(data []byte) *Program {
	program, err := ProgramFromJSON(data)
	if err != nil {
		panic(err)
	}
	return program
}

func (p *Program) ToJSON() ([]byte, error) {
	return json.Marshal(p)
}

func (p *Program) Validate() error {
	if len(p.Procedures) == 0 {
		return fmt.Errorf("program has no procedures")
	}
	if p.Entry == "" {
		p.Entry = p.Procedures[0].Name
	}
	seenSymbols := map[string]struct{}{}
	for i := range p.Symbols {
		name := strings.ToLower(p.Symbols[i].Name)
		if name == "" {
			return fmt.Errorf("symbol %d has no name", i)
		}
		if _, exists := seenSymbols[name]; exists {
			return fmt.Errorf("duplicate symbol %q", p.Symbols[i].Name)
		}
		seenSymbols[name] = struct{}{}
	}
	seenProcs := map[string]struct{}{}
	for i := range p.Procedures {
		name := strings.ToLower(p.Procedures[i].Name)
		if name == "" {
			return fmt.Errorf("procedure %d has no name", i)
		}
		if _, exists := seenProcs[name]; exists {
			return fmt.Errorf("duplicate procedure %q", p.Procedures[i].Name)
		}
		seenProcs[name] = struct{}{}
		if p.Procedures[i].Labels == nil {
			p.Procedures[i].Labels = map[string]int{}
		}
	}
	return nil
}
